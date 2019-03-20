package natter

import (
	"errors"
	"fmt"
	"github.com/lucas-clemente/quic-go"
	"github.com/songgao/water"
	"heckel.io/natter/internal"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"time"
)

func (c *client) ForwardL2(localNetwork string, target string, targetNetwork string) (Forward, error) {
	return c.forward(forwardTypeL2, "", localNetwork, target, targetNetwork, "", []string{})
}

func (c *client) ForwardTCP(localAddr string, target string, targetForwardAddr string, targetCommand []string) (Forward, error) {
	return c.forward(forwardTypeTCP, localAddr, "", target, "", targetForwardAddr, targetCommand)
}

func (c *client) forward(ftype string, localAddr string, localNetwork string,
	target string, targetNetwork string, targetForwardAddr string, targetCommand []string) (Forward, error) {

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
		mode:              ftype,
		source:            c.config.ClientId,
		sourceAddr:        localAddr,
		sourceNetwork:     localNetwork,
		target:            target,
		targetNetwork:     targetNetwork,
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
		SourceAddr:        forward.sourceAddr,
		SourceNetwork:     forward.sourceNetwork,
		Target:            forward.target,
		TargetNetwork:     forward.targetNetwork,
		TargetForwardAddr: forward.targetForwardAddr,
		TargetCommand:     forward.targetCommand,
	})
	if err != nil {
		return nil, err
	}

	return forward, nil
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
		targetNetwork := forward.targetNetwork
		forward.Unlock()

		config := water.Config{
			DeviceType: water.TAP,
		}

		config.Name = "tap" + forward.id + "0"

		log.Printf("Listening on tap%s\n", config.Name)

		ifce, err := water.New(config)
		if err != nil {
			log.Printf("Failed to create tap device %s: %s\n", config.Name, err.Error())
			return
		}

		if out, err := exec.Command("ip", "link", "set", config.Name, "up").CombinedOutput(); err != nil {
			log.Printf("Failed to up link %s: %s\n%s\n", config.Name, err.Error(), out)
			return
		}

		if out, err := exec.Command("ip", "route", "add", targetNetwork, "dev", config.Name).CombinedOutput(); err != nil {
			log.Printf("Failed to add route %s: %s\n%s\n", targetNetwork, err.Error(), out)
			return
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
	forward.Unlock()

	if forward.mode == forwardTypeL2 {
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
