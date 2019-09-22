package natter

import (
	"errors"
	"fmt"
	"github.com/lucas-clemente/quic-go"
	"github.com/songgao/water"
	"github.com/vishvananda/netlink"
	"heckel.io/natter/internal"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"time"
)

func (c *client) Bridge(localBridge string, localRoutes []string, localDhcp bool,
	target string, targetBridge string, targetRoutes []string) (Forward, error) {

	return c.forward(forwardModeBridge, "", localBridge, localDhcp, localRoutes,
		target, targetBridge, targetRoutes, "", []string{})
}

func (c *client) Forward(localAddr string, target string,
	targetForwardAddr string, targetCommand []string) (Forward, error) {

	return c.forward(forwardModeTcp, localAddr, "", false, []string{},
		target, "", []string{}, targetForwardAddr, targetCommand)
}

func (c *client) forward(mode string, localAddr string, localBridge string, localDhcp bool, localRoutes []string,
	target string, targetBridge string, targetRoutes []string, targetForwardAddr string, targetCommand []string) (Forward, error) {

	log.Printf("Adding forward from local address %s to %s %s\n", localAddr, target, targetForwardAddr)

	if target == c.config.ClientId {
		return nil, errors.New("cannot forward to yourself")
	}

	err := c.conn.connect()
	if err != nil {
		return nil, errors.New("cannot connect to broker: " + err.Error())
	}

	// Create forward entry
	forward := &forward{
		id:                c.generateConnId(),
		mode:              mode,
		source:            c.config.ClientId,
		sourceAddr:        localAddr,
		sourceDhcp:        localDhcp,
		sourceBridge:      localBridge,
		sourceRoutes:      localRoutes,
		target:            target,
		targetBridge:      targetBridge,
		targetRoutes:      targetRoutes,
		targetForwardAddr: targetForwardAddr,
		targetCommand:     targetCommand,
	}

	c.forwardsMutex.Lock()
	defer c.forwardsMutex.Unlock()

	c.forwards[forward.id] = forward

	// Sending forward request
	log.Printf("Requesting connection to target %s on TCP address %s\n", target, targetForwardAddr)
	err = c.conn.Send(messageTypeForwardRequest, &internal.ForwardRequest{
		Id:                forward.id,
		Mode:              forward.mode,
		Source:            forward.source,
		SourceBridge:      forward.sourceBridge,
		SourceDhcp:        forward.sourceDhcp,
		SourceAddr:        forward.sourceAddr,
		SourceRoutes:      forward.sourceRoutes,
		Target:            forward.target,
		TargetBridge:      forward.targetBridge,
		TargetRoutes:      forward.targetRoutes,
		TargetForwardAddr: forward.targetForwardAddr,
		TargetCommand:     forward.targetCommand,
	})
	if err != nil {
		return nil, err
	}

	return forward, nil
}


func (c *client) handleForwardResponse(response *internal.ForwardResponse) {
	c.forwardsMutex.Lock()
	defer c.forwardsMutex.Unlock()

	forward, ok := c.forwards[response.Id]

	if !ok {
		log.Println("Forward response with invalid ID received. Ignoring.")
		return
	}

	if !response.Success {
		log.Println("Failed forward response")
		return
	}

	var err error
	log.Print("Peer address: ", response.TargetAddr)

	forward.Lock()
	forward.peerUdpAddr, err = net.ResolveUDPAddr("udp4", response.TargetAddr)
	if forward.mode == forwardModeBridge && len(response.SourceRoutesDiscovered) > 0 {
		forward.sourceRoutes = response.SourceRoutesDiscovered
	}
	forward.Unlock()

	if forward.mode == forwardModeBridge {
		c.forwardFromTap(forward)
	} else {
		// Listen to local TCP address
		if forward.sourceAddr == "" {
			c.forwardFromStdin(forward)
		} else {
			if err := c.forwardFromTcp(forward); err != nil {
				log.Println("Failed to forward from TCP")
				return
			}
		}
	}

	if err != nil {
		// TODO close forward
		log.Println("Failed to resolve peer UDP address: " + err.Error())
		return
	}

	go c.punch(forward.peerUdpAddr)
}

func (c *client) forwardFromStdin(forward *forward) {
	log.Println("Reading from STDIN")

	rw := struct {
		io.Reader
		io.Writer
	} {
		os.Stdin,
		os.Stdout,
	}

	go c.openPeerStream(forward, rw)
}

func (c *client) forwardFromTcp(forward *forward) error {
	log.Printf("Listening on local TCP address %s\n", forward.sourceAddr)

	localTcpListener, err := net.Listen("tcp", forward.sourceAddr)
	if err != nil {
		return err
	}

	go c.listenTcp(forward, localTcpListener)
	return nil
}

func (c *client) forwardFromTap(forward *forward) {
	go func() {
		forward.Lock()
		sourceRoutes := forward.sourceRoutes
		sourceDhcp := forward.sourceDhcp
		forward.Unlock()

		tapName := "tap" + forward.id + "0"

		config := water.Config{DeviceType: water.TAP}
		config.Name = tapName

		log.Printf("Listening on tap%s\n", tapName)

		ifce, err := water.New(config)
		if err != nil {
			log.Printf("Failed to create tap device %s: %s\n", tapName, err.Error())
			return
		}

		tapLink, err := netlink.LinkByName(tapName)
		if err != nil {
			log.Printf("Failed get link %s: %s\n", tapName, err.Error())
			return
		}

		if err := netlink.LinkSetUp(tapLink); err != nil {
			log.Printf("Failed to up link %s: %s\n", tapName, err.Error())
			return
		}

		for _, dstRoute := range sourceRoutes {
			log.Println("Adding route " + dstRoute)

			_, dstNet, err := net.ParseCIDR(dstRoute)
			if err != nil {
				log.Printf("Cannot add route %s: %s\n", dstRoute, err.Error())
				return
			}

			err = netlink.RouteAdd(&netlink.Route{
				LinkIndex: tapLink.Attrs().Index,
				Dst:       dstNet,
			})
			if err != nil {
				log.Printf("Cannot add route %s: %s\n", dstRoute, err.Error())
				return
			}
		}

		if sourceDhcp {
			go func() { // TODO At this point the tap is not connected, so this needs to run asynchronoiusly or later
				log.Printf("Running DHCP on tap interface ...")

				cmd := exec.Command("dhclient", "-v", tapName)
				err := cmd.Run()
				if err != nil {
					log.Printf("Cannot get DHCP address for tap interface: %s\n", err.Error())
					return
				}

				// Remove default route (dhclient adds one ...)
				routes, err := netlink.RouteList(tapLink, netlink.FAMILY_V4)
				if err != nil {
					log.Printf("Cannot get list of routes: %s\n", err.Error())
					return
				}

				for _, route := range routes {
					//log.Printf("checking route %#v", route)
					defaultRoute := route.Dst == nil && route.Src == nil
					if defaultRoute {
						log.Printf("Removing default route to tap (dhclient added this ...)")
						err := netlink.RouteDel(&route)
						if err != nil {
							log.Printf("Cannot remove default route %#v: %s\n", route, err.Error())
							return
						}
					}
				}
			}()
		}

		go c.openPeerStream(forward, ifce)
	}()
}

func (c *client) generateConnId() string {
	b := make([]byte, connectionIdLength)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func (c *client) listenTcp(forward *forward, listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Accepting TCP connection failed: " + err.Error())
			continue
		}

		go c.openPeerStream(forward, conn)
	}
}

func (c *client) openPeerStream(forward *forward, localStream io.ReadWriter) {
	log.Print("Opening stream to peer")

	var peerStream quic.Stream

	// TODO fix this ugly wait loop
	for forward.PeerUdpAddr() == nil {
		log.Println("Client connected on TCP socket, opening stream ...")
		time.Sleep(1 * time.Second)
	}

	for {
		peerUdpAddr := forward.PeerUdpAddr()
		tlsClientConfig := *c.config.TLSClientConfig // copy, because quic-go alters it!
		sniHost := fmt.Sprintf("%s:%d", forward.id, 2586) // Connection ID in the SNI host, port doesn't matter!
		session, err := quic.Dial(c.conn.UdpConn(), peerUdpAddr, sniHost, &tlsClientConfig, c.config.QuicConfig)

		if err != nil {
			log.Println("Cannot connect to remote peer via " + peerUdpAddr.String() + ". Closing.")
			return // TODO close forward
		}

		peerStream, err = session.OpenStreamSync()

		if err != nil {
			log.Println("Not connected yet.")
			time.Sleep(1 * time.Second)
			continue
		}

		break
	}

	log.Println("Connected. Starting to forward.")

	go func() { io.Copy(peerStream, localStream) }()
	go func() { io.Copy(localStream, peerStream) }()
}
