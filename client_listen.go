package natter

import (
	"errors"
	"github.com/lucas-clemente/quic-go"
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

	listener, err := quic.Listen(c.conn.UdpConn(), c.config.TLSConfig, c.config.QuicConfig) // TODO
	if err != nil {
		return errors.New("cannot listen on UDP socket for incoming connections")
	}

	go c.handleIncomingPeers(listener)

	return nil
}

func (c *client) handleForwardRequest(request *internal.ForwardRequest) {
	// TODO ignore if not in "daemon mode"

	log.Printf("Accepted forward request from %s to TCP addr %s", request.Source, request.TargetForwardAddr)

	peerUdpAddr, err := net.ResolveUDPAddr("udp4", request.SourceAddr)
	if err != nil {
		log.Println("Cannot resolve peer udp addr: " + err.Error())
		c.conn.Send(messageTypeForwardResponse, &internal.ForwardResponse{Success: false})
		return // TODO close forward
	}

	forward := &forward{
		id: request.Id,
		source: request.Source,
		sourceAddr: request.SourceAddr,
		target: request.Target,
		targetForwardAddr: request.TargetForwardAddr,
		targetCommand: request.TargetCommand,
		peerUdpAddr: peerUdpAddr,
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
	peerAddr := session.RemoteAddr().(*net.UDPAddr)
	connectionId := session.ConnectionState().ServerName // Connection ID is the SNI host!

	c.forwardsMutex.Lock()

	forward, ok := c.forwards[connectionId]
	if !ok {
		log.Printf("Cannot find forward for connection ID %s. Closing.", connectionId)
		session.Close()
		c.forwardsMutex.Unlock()
		return
	}

	targetForwardAddr := forward.targetForwardAddr
	targetCommand := forward.targetCommand

	if targetCommand != nil && len(targetCommand) > 0 {
		log.Println("Client accepted from " + peerAddr.String() + ", forward found to command " + strings.Join(forward.targetCommand, " "))
	} else {
		log.Println("Client accepted from " + peerAddr.String() + ", forward found to " + targetForwardAddr)
	}

	c.forwardsMutex.Unlock()

	for {
		stream, err := session.AcceptStream()
		if err != nil {
			log.Println("Failed to accept peer stream. Closing session: " + err.Error())
			session.Close()
			break
		}

		go c.handlePeerStream(session, stream, targetForwardAddr, targetCommand)
	}
}

func (c *client) handlePeerStream(session quic.Session, stream quic.Stream, targetForwardAddr string, targetCommand []string) {
	log.Printf("Stream %d accepted. Starting to forward.\n", stream.StreamID())

	if targetCommand != nil && len(targetCommand) > 0 {
		c.forwardToCommand(stream, targetCommand)
	} else {
		c.forwardToTcp(stream, targetForwardAddr)
	}
}

func (c *client) forwardToCommand(stream quic.Stream, targetCommand []string) {
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

func (c *client) forwardToTcp(stream quic.Stream, targetForwardAddr string) {
	forwardStream, err := net.Dial("tcp", targetForwardAddr)
	if err != nil {
		log.Printf("Cannot open connection to %s: %s\n", targetForwardAddr, err.Error())
		return // TODO close forward
	}

	go func() { io.Copy(stream, forwardStream) }()
	go func() { io.Copy(forwardStream, stream) }()
}

