package natter

import (
	"errors"
	"fmt"
	"github.com/lucas-clemente/quic-go"
	"heckel.io/natter/internal"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"time"
)

func (c *client) Forward(localAddr string, target string, targetForwardAddr string, targetCommand []string) (Forward, error) {
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
		source:            c.config.ClientId,
		sourceAddr:        localAddr,
		target:            target,
		targetForwardAddr: targetForwardAddr,
		targetCommand:     targetCommand,
	}

	c.forwardsMutex.Lock()
	defer c.forwardsMutex.Unlock()

	c.forwards[forward.id] = forward

	// Listen to local TCP address
	if localAddr == "" {
		c.forwardFromStdin(forward)
	} else {
		if err := c.forwardFromTcp(forward); err != nil {
			return nil, err
		}
	}

	// Sending forward request
	log.Printf("Requesting connection to target %s on TCP address %s\n", target, targetForwardAddr)
	err = c.conn.Send(messageTypeForwardRequest, &internal.ForwardRequest{
		Id:                forward.id,
		Source:            forward.source,
		Target:            forward.target,
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

	if err != nil {
		// TODO close forward
		log.Println("Failed to resolve peer UDP address: " + err.Error())
		return
	}

	go c.punch(forward.peerUdpAddr)
}
