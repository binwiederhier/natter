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

func (client *client) Forward(localAddr string, target string, targetForwardAddr string, targetCommand []string) (Forward, error) {
	log.Printf("Adding forward from local address %s to %s %s\n", localAddr, target, targetForwardAddr)

	err := client.conn.connect()
	if err != nil {
		return nil, errors.New("cannot connect to broker: " + err.Error())
	}

	// Create forward entry
	forward := &forward{
		id: client.generateConnId(),
		source: client.config.ClientUser,
		sourceAddr: localAddr,
		target: target,
		targetForwardAddr: targetForwardAddr,
		targetCommand: targetCommand,
	}

	client.forwardsMutex.Lock()
	defer client.forwardsMutex.Unlock()

	client.forwards[forward.id] = forward

	// Listen to local TCP address
	if localAddr == "" {
		log.Println("Reading from STDIN")

		rw := struct {
			io.Reader
			io.Writer
		} {
			os.Stdin,
			os.Stdout,
		}

		go client.openPeerStream(forward, rw)
	} else {
		log.Printf("Listening on local TCP address %s\n", localAddr)
		localTcpListener, err := net.Listen("tcp", localAddr)
		if err != nil {
			return nil, err
		}

		go client.listenTcp(forward, localTcpListener)
	}

	// Sending forward request
	log.Printf("Requesting connection to target %s on TCP address %s\n", target, targetForwardAddr)
	err = client.conn.Send(messageTypeForwardRequest, &internal.ForwardRequest{
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

func (client *client) generateConnId() string {
	b := make([]byte, connectionIdLength)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}


func (client *client) listenTcp(forward *forward, listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Accepting TCP connection failed: " + err.Error())
			continue
		}

		go client.openPeerStream(forward, conn)
	}
}

func (client *client) openPeerStream(forward *forward, localStream io.ReadWriter) {
	log.Print("Opening stream to peer")

	var peerStream quic.Stream

	// TODO fix this ugly wait loop
	for forward.PeerUdpAddr() == nil {
		log.Println("Client connected on TCP socket, opening stream ...")
		time.Sleep(1 * time.Second)
	}

	for {
		peerUdpAddr := forward.PeerUdpAddr()
		sniHost := fmt.Sprintf("%s:%d", forward.id, 2586) // Connection ID in the SNI host, port doesn't matter!
		session, err := quic.Dial(client.conn.UdpConn(), peerUdpAddr, sniHost, generateQuicTlsClientConfig(),
			generateQuicConfig()) // TODO fix this

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

func (client *client) handleForwardResponse(response *internal.ForwardResponse) {
	client.forwardsMutex.Lock()
	defer client.forwardsMutex.Unlock()

	forward, ok := client.forwards[response.Id]

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

	forward.peerUdpAddr, err = net.ResolveUDPAddr("udp4", response.TargetAddr)
	if err != nil {
		// TODO close forward
		return
	}

	go client.punch(forward.peerUdpAddr)
}

func (client *client) punch(udpAddr *net.UDPAddr) {
	// TODO add exitChan support!!

	for {
		udpConn := client.conn.UdpConn()

		if udpConn != nil {
			udpConn.WriteTo([]byte("punch!"), udpAddr)
		}

		time.Sleep(punchInterval)
	}
}
