package natter

import (
	"errors"
	"github.com/lucas-clemente/quic-go"
	"github.com/songgao/water"
	"heckel.io/natter/internal"
	"io"
	"log"
	"net"
	"os/exec"
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
		sourceNetwork:     request.SourceNetwork,
		target:            request.Target,
		targetNetwork:     request.TargetNetwork,
		targetForwardAddr: request.TargetForwardAddr,
		targetCommand:     request.TargetCommand,
		peerUdpAddr:       peerUdpAddr,
	}

	c.forwardsMutex.Lock()
	defer c.forwardsMutex.Unlock()

	c.forwards[request.Id] = forward

	err = c.conn.Send(messageTypeForwardResponse, &internal.ForwardResponse{
		Id:         request.Id,
		Success:    true,
		Source:     request.Source,
		SourceAddr: request.SourceAddr,
		Target:     request.Target,
		TargetAddr: request.TargetAddr,
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

	if ftype == forwardTypeL2 {
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
	sourceNetwork := forward.sourceNetwork
	forward.Unlock()

	log.Println("Forwarding stream from " + peerUdpAddr.String() + " to tap interface for network " + sourceNetwork)

	config := water.Config{
		DeviceType: water.TAP,
	}

	config.Name = "tap" + forward.id + "1"

	ifce, err := water.New(config)
	if err != nil {
		log.Fatal(err)
		return
	}

	if out, err := exec.Command("ip", "link", "set", config.Name, "up").CombinedOutput(); err != nil {
		log.Printf("Failed to up link %s: %s\n%s\n", config.Name, err.Error(), out)
		return
	}

	if out, err := exec.Command("ip", "route", "add", sourceNetwork, "dev", config.Name).CombinedOutput(); err != nil {
		log.Printf("Failed to add route %s: %s\n%s\n", sourceNetwork, err.Error(), out)
		return
	}

	go func() { io.Copy(stream, ifce) }()
	go func() { io.Copy(ifce, stream) }()
}


