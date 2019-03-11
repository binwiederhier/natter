package natter

import (
	"errors"
	"fmt"
	"github.com/lucas-clemente/quic-go"
	"io"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"
)

type client struct {
	clientId string
	config   *ClientConfig
	serverAddr   *net.UDPAddr
	serverStream quic.Stream
	udpConn net.PacketConn
	mu sync.RWMutex
	incomingForwards map[string]*inforward
	outgoingForwards map[string]*outforward
}

type inforward struct {
	connectionId string
	peerUdpAddr  *net.UDPAddr
	targetForwardAddr string
}

type outforward struct {
	mu sync.RWMutex
	peerUdpAddr  *net.UDPAddr
	status string

	id string
	source string
	sourceAddr string
	target string
	targetForwardAddr string
}

func (outforward *outforward) Status() string {
	outforward.mu.RLock()
	defer outforward.mu.RUnlock()
	return outforward.status
}

type ClientConfig struct {

}

func NewClient(clientId string, serverAddr string, config *ClientConfig) (*client, error) {
	rand.Seed(time.Now().UTC().UnixNano())

	if clientId == "" {
		return nil, errors.New("client identifier cannot be empty")
	}

	udpServerAddr, err := net.ResolveUDPAddr("udp4", serverAddr)
	if err != nil {
		return nil, err
	}

	if config == nil {
		config = &ClientConfig{}
	}

	return &client{
		clientId:   clientId,
		config:     config,
		serverAddr: udpServerAddr,
		serverStream: nil,
		udpConn: nil,
		incomingForwards: make(map[string]*inforward),
		outgoingForwards: make(map[string]*outforward),
	}, nil
}

func (client *client) Forward(localAddr string, target string, targetForwardAddr string) (*outforward, error) {
	err := client.connectToServer()
	if err != nil {
		return nil, errors.New("cannot connect to server: " + err.Error())
	}

	// Create forward entry
	forward := &outforward{
		id: createRandomString(8),
		status: "created", // TODO
		source: client.clientId,
		sourceAddr: localAddr,
		target: target,
		targetForwardAddr: targetForwardAddr,
	}

	client.mu.Lock()
	defer client.mu.Unlock()

	client.outgoingForwards[forward.id] = forward

	// Listen to local TCP address
	log.Printf("Listening on local TCP address %s\n", localAddr)
	localTcpListener, err := net.Listen("tcp", localAddr)
	if err != nil {
		return nil, err
	}

	go client.listenTcp(forward, localTcpListener)

	// Sending forward request
	log.Printf("Requesting connection to target %s on TCP address %s\n", target, targetForwardAddr)
	sendMessage(client.serverStream, messageTypeForwardRequest, &ForwardRequest{
		Id:                forward.id,
		Source:            forward.source,
		Target:            forward.target,
		TargetForwardAddr: forward.targetForwardAddr,
	})

	return forward, nil
}

func (client *client) connectToServer() error {
	var err error

	// Check if already connected
	if client.serverStream != nil {
		return nil
	}

	// Listen to local UDP address
	udpAddr := fmt.Sprintf(":%d", 10000+rand.Intn(10000))
	log.Printf("Listening on UDP address %s\n", udpAddr)

	client.udpConn, err = net.ListenPacket("udp", udpAddr)
	if err != nil {
		return err
	}

	log.Printf("Connecting to server at %s\n", client.serverAddr.String())
	session, err := quic.Dial(client.udpConn, client.serverAddr, client.serverAddr.String(),
		generateQuicTlsClientConfig(), generateQuicConfig()) // TODO fix this
	if err != nil {
		return err
	}

	client.serverStream, err = session.OpenStream()
	if err != nil {
		return err
	}

	go client.listenServerStream()

	return nil
}

func (client *client) listenServerStream() {
	for {
		messageType, message := receiveMessage(client.serverStream)

		switch messageType {
		case messageTypeRegisterResponse:
			client.handleRegisterResponse(message.(*RegisterResponse))
		case messageTypeForwardRequest:
			client.handleForwardRequest(message.(*ForwardRequest))
		case messageTypeForwardResponse:
			client.handleForwardResponse(message.(*ForwardResponse))
		default:
			log.Println("Unknown message type", int(messageType))
		}
	}
}

func (client *client) listenTcp(forward *outforward, listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Accepting TCP connection failed: " + err.Error())
			continue
		}

		log.Println("Client connected on TCP socket, opening stream ...")
		go client.openPeerStream(forward, conn)
	}
}

func (client *client) openPeerStream(forward *outforward, conn net.Conn) {
	log.Print("[forwarder] Opening stream to peer")

	var peerUdpAddr *net.UDPAddr
	var peerStream quic.Stream

	for {
		forward.mu.RLock()
		peerUdpAddr = forward.peerUdpAddr
		forward.mu.RUnlock()

		if peerUdpAddr != nil {
			break
		} else {
			log.Println("Client connected on TCP socket, opening stream ...")
			time.Sleep(1 * time.Second)
		}
	}

	for {
		peerHost := fmt.Sprintf("%s:%d", peerUdpAddr.IP.String(), peerUdpAddr.Port)
		session, err := quic.Dial(client.udpConn, peerUdpAddr, peerHost, generateQuicTlsClientConfig(),
			generateQuicConfig()) // TODO fix this

		if err != nil {
			panic(err) // FIXME
		}

		peerStream, err = session.OpenStreamSync()

		if err != nil {
			log.Println("[forwarder] Not connected yet.")
			time.Sleep(1 * time.Second)
			continue
		}

		break
	}

	log.Println("[forwarder] Connected. Starting to forward.")

	go func() { io.Copy(peerStream, conn) }()
	go func() { io.Copy(conn, peerStream) }()
}

func (client *client) handleRegisterResponse(response *RegisterResponse) {
	// Nothing.
}

func (client *client) handleForwardRequest(request *ForwardRequest) {
	// TODO ignore if not in "daemon mode"

	log.Printf("Accepted forward request from %s to TCP addr %s", request.Source, request.TargetForwardAddr)

	peerUdpAddr, err := net.ResolveUDPAddr("udp4", request.SourceAddr)
	if err != nil {
		log.Println("Cannot resolve peer udp addr: " + err.Error())
		sendMessage(client.serverStream, messageTypeForwardResponse, &ForwardResponse{Success: false})
		return
	}

	forward := &inforward{
		connectionId: request.Id,
		targetForwardAddr: request.TargetForwardAddr,
		peerUdpAddr: peerUdpAddr,
	}

	client.incomingForwards[peerUdpAddr.String()] = forward

	sendMessage(client.serverStream, messageTypeForwardResponse, &ForwardResponse{
		Id:         request.Id,
		Success:    true,
		Source:     request.Source,
		SourceAddr: request.SourceAddr,
		Target:     request.Target,
		TargetAddr: request.TargetAddr,
	})

	// TODO add proper method, add doneChan
	go client.punch(peerUdpAddr)
}

func (client *client) handleForwardResponse(response *ForwardResponse) {
	client.mu.Lock()
	defer client.mu.Unlock()

	forward, ok := client.outgoingForwards[response.Id]

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
		forward.status = "error " + err.Error() // TODO fix me
		return
	}

	forward.status = "success"

	go client.punch(forward.peerUdpAddr)
}

func (client *client) punch(udpAddr *net.UDPAddr) {
	// TODO add doneChan support!!

	for {
		client.udpConn.WriteTo([]byte("punch!"), udpAddr)
		time.Sleep(15 * time.Second)
	}
}

