package natter

import (
	"errors"
	"github.com/lucas-clemente/quic-go"
	"github.com/songgao/water"
	"github.com/vishvananda/netlink"
	"heckel.io/natter/internal"
	"io"
	"log"
	"net"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

func (c *client) Listen() error {
	err := c.conn.connect()
	if err != nil {
		return errors.New("cannot connect to broker: " + err.Error())
	}

	listener, err := quic.Listen(c.conn.UdpConn(), c.config.TLSServerConfig, c.config.QuicConfig) // TODO
	if err != nil {
		return errors.New("cannot listen on UDP socket for incoming connections:" + err.Error())
	}

	go c.handleIncomingPeers(listener)

	return nil
}

func (c *client) handleForwardRequest(request *internal.ForwardRequest) {
	// TODO ignore if not in "daemon mode"

	log.Printf("Accepted forward request from %s to %s", request.Source, request.TargetForwardAddr)

	peerUdpAddr, err := net.ResolveUDPAddr("udp4", request.SourceAddr)
	if err != nil {
		log.Println("Cannot resolve peer udp addr: " + err.Error())
		if err := c.conn.Send(messageTypeForwardResponse, &internal.ForwardResponse{Success: false}); err != nil {
			log.Println("Cannot send forward response: " + err.Error())
		}
		return // TODO close forward
	}

	forward := &forward{
		id:                request.Id,
		mode:              request.Mode,
		source:            request.Source,
		sourceAddr:        request.SourceAddr,
		sourceDhcp:        request.SourceDhcp,
		sourceRoutes:      request.SourceRoutes,
		sourceBridge:      request.SourceBridge,
		target:            request.Target,
		targetRoutes:      request.TargetRoutes,
		targetBridge:      request.TargetBridge,
		targetForwardAddr: request.TargetForwardAddr,
		targetCommand:     request.TargetCommand,
		peerUdpAddr:       peerUdpAddr,
	}

	c.forwardsMutex.Lock()
	defer c.forwardsMutex.Unlock()

	c.forwards[request.Id] = forward

	var sourceRoutesDiscovered = make([]string, 0)

	if request.Mode == forwardModeBridge {
		if len(request.SourceRoutes) == 1 && request.SourceRoutes[0] == "auto" {
			var out []byte

			if out, err = exec.Command("ip", "route", "get", "1").Output(); err != nil {
				log.Printf("Failed to get default route: %s\n", err.Error())
				return
			}

			line := string(out)
			defaultDeviceRegex := regexp.MustCompile(`\bdev\s+(\S+)`)
			matches := defaultDeviceRegex.FindStringSubmatch(line)

			if len(matches) != 2 {
				log.Printf("Failed to get default route, cannot match line: %s\n%s\n", err.Error(), line)
				return
			}

			defaultDevice := matches[1]

			if out, err = exec.Command("ip", "-o", "-f", "inet", "addr", "show", defaultDevice).CombinedOutput(); err != nil {
				log.Printf("Failed to get default route: %s\n%s\n", err.Error(), out)
				return
			}

			line = strings.TrimSpace(string(out))
			defaultNetworkRegex := regexp.MustCompile(`\binet\s+(\S+)`)
			matches = defaultNetworkRegex.FindStringSubmatch(line)

			if len(matches) != 2 {
				log.Printf("Failed to get default route, cannot match line: %s\n%s\n", err.Error(), line)
				return
			}

			defaultNetwork := matches[1]

			sourceRoutesDiscovered = append(sourceRoutesDiscovered, defaultNetwork)
		}
	}

	err = c.conn.Send(messageTypeForwardResponse, &internal.ForwardResponse{
		Id:                     request.Id,
		Success:                true,
		Source:                 request.Source,
		SourceAddr:             request.SourceAddr,
		SourceRoutesDiscovered: sourceRoutesDiscovered,
		Target:                 request.Target,
		TargetAddr:             request.TargetAddr,
	})
	if err != nil {
		log.Println("Cannot send forward response: " + err.Error())
		return // TODO close forward
	}

	go c.punch(peerUdpAddr)
}

func (c *client) handleIncomingPeers(listener quic.Listener) {
	for {
		log.Println("Waiting for connections")
		session, err := listener.Accept()
		if err != nil {
			log.Println("Cannot accept peer connections: " + err.Error())
			time.Sleep(5 * time.Second)
			continue
		}

		go c.handlePeerSession(session)
	}
}

func (c *client) handlePeerSession(session quic.Session) {
	log.Println("Session from " + session.RemoteAddr().String() + " accepted.")
	connectionId := session.ConnectionState().ServerName // Connection ID is the SNI host!

	c.forwardsMutex.Lock()
	forward, ok := c.forwards[connectionId]
	c.forwardsMutex.Unlock()

	if !ok {
		log.Printf("Cannot find forward for connection ID %s. Closing.", connectionId)
		session.Close()
		return
	}

	for {
		stream, err := session.AcceptStream()
		if err != nil {
			log.Println("Failed to accept peer stream. Closing session: " + err.Error())
			session.Close()
			break
		}

		go c.handlePeerStream(session, stream, forward)
	}
}

func (c *client) handlePeerStream(session quic.Session, stream quic.Stream, forward *forward) {
	log.Printf("Stream %d accepted. Starting to forward.\n", stream.StreamID())

	forward.Lock()
	ftype := forward.mode
	targetCommand := forward.targetCommand
	forward.Unlock()

	if ftype == forwardModeBridge {
		c.forwardToTap(stream, forward)
	} else {
		if targetCommand != nil && len(targetCommand) > 0 {
			c.forwardToCommand(stream, forward)
		} else {
			c.forwardToTcp(stream, forward)
		}
	}
}

func (c *client) forwardToCommand(stream quic.Stream, forward *forward) {
	forward.Lock()
	peerUdpAddr := forward.peerUdpAddr
	targetCommand := forward.targetCommand
	forward.Unlock()

	log.Println("Forwarding stream from " + peerUdpAddr.String() + ", forward found to command " + strings.Join(targetCommand, " "))
	cmd := exec.Command(targetCommand[0], targetCommand[1:]...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Println(err.Error())
		return // TODO close forward
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Println(err.Error())
		return // TODO close forward
	}

	err = cmd.Start()
	if err != nil {
		log.Println(err.Error())
		return // TODO close forward
	}

	go func() { io.Copy(stream, stdout) }()
	go func() { io.Copy(stdin, stream) }()
}

func (c *client) forwardToTcp(stream quic.Stream, forward *forward) {
	forward.Lock()
	peerUdpAddr := forward.peerUdpAddr
	targetForwardAddr := forward.targetForwardAddr
	forward.Unlock()

	log.Println("Forwarding stream from " + peerUdpAddr.String() + " to TCP address " + targetForwardAddr)

	forwardStream, err := net.Dial("tcp", targetForwardAddr)
	if err != nil {
		log.Printf("Cannot open connection to %s: %s\n", targetForwardAddr, err.Error())
		return // TODO close forward
	}

	go func() { io.Copy(stream, forwardStream) }()
	go func() { io.Copy(forwardStream, stream) }()
}

func (c *client) forwardToTap(stream quic.Stream, forward *forward) {
	forward.Lock()
	peerUdpAddr := forward.peerUdpAddr
	targetRoutes := forward.targetRoutes
	targetBridge := forward.targetBridge
	forward.Unlock()

	log.Println("Forwarding stream from " + peerUdpAddr.String() + " to tap interface")

	tapName := "tap" + forward.id + "1"
	config := water.Config{DeviceType: water.TAP}
	config.Name = tapName

	ifce, err := water.New(config)
	if err != nil {
		log.Fatal(err)
		return
	}

	tapLink, err := netlink.LinkByName(tapName)
	if err != nil {
		log.Printf("Failed get link %s: %s\n", tapName, err.Error())
		return
	}

	if targetBridge != "" {
		bridgeLink, err := netlink.LinkByName(targetBridge)
		if err != nil {
			log.Printf("Failed get link for bridge %s: %s\n", targetBridge, err.Error())
			return
		}

		err = netlink.LinkSetMasterByIndex(tapLink, bridgeLink.Attrs().Index)
		if err != nil {
			log.Printf("Failed to add tap to target bridge %s: %s\n", targetBridge, err.Error())
			return
		}
	}

	if err := netlink.LinkSetUp(tapLink); err != nil {
		log.Printf("Failed to up link %s: %s\n", tapName, err.Error())
		return
	}

	// TODO duplicate code
	for _, dstRoute := range targetRoutes {
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

	go func() { io.Copy(stream, ifce) }()
	go func() { io.Copy(ifce, stream) }()
}


