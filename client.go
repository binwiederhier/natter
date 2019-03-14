package natter

// TODO allow forwarding from STDIN
// TODO allow forwarding to remote command

import (
	"errors"
	"fmt"
	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/qerr"
	"io"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"
)

type Client struct {
	config           *ClientConfig
	brokerAddr       *net.UDPAddr
	brokerMessenger  *messenger
	brokerMutex      sync.RWMutex
	udpConn          net.PacketConn
	forwards         map[string]*forward
	forwardsMutex    sync.RWMutex
}

type forward struct {
	peerUdpAddr       *net.UDPAddr
	status            string
	id                string
	source            string
	sourceAddr        string
	target            string
	targetForwardAddr string

	sync.RWMutex
}

func (forward *forward) PeerUdpAddr() *net.UDPAddr {
	forward.RLock()
	defer forward.RUnlock()
	return forward.peerUdpAddr
}

type ClientConfig struct {
	ClientUser string
	BrokerAddr string
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// NewClient creates a new client struct. It checks the configuration
// passed and returns an error if it is invalid.
func NewClient(config *ClientConfig) (*Client, error) {
	rand.Seed(time.Now().UTC().UnixNano())

	if config.ClientUser == "" {
		return nil, errors.New("invalid config: ClientUser cannot be empty")
	}

	if config.BrokerAddr == "" {
		return nil, errors.New("invalid config: ServerAddr cannot be empty")
	}

	udpBrokerAddr, err := net.ResolveUDPAddr("udp4", config.BrokerAddr)
	if err != nil {
		return nil, err
	}

	return &Client{
		config:           config,
		brokerAddr:       udpBrokerAddr,
		brokerMessenger:  nil,
		udpConn:          nil,
		forwards:         make(map[string]*forward),
	}, nil
}

func LoadClientConfig(filename string) (*ClientConfig, error) {
	rawconfig, err := loadRawConfig(filename)
	if err != nil {
		return nil, err
	}

	clientUser, ok := rawconfig["ClientUser"]
	if !ok {
		return nil, errors.New("invalid config file, ClientUser setting is missing")
	}

	brokerAddr, ok := rawconfig["BrokerAddr"]
	if !ok {
		return nil, errors.New("invalid config file, Server setting is missing")
	}

	return &ClientConfig{
		ClientUser: clientUser,
		BrokerAddr: brokerAddr,
	}, nil
}

func (client *Client) ListenIncoming() error {
	err := client.connectToServer()
	if err != nil {
		return errors.New("cannot connect to broker: " + err.Error())
	}

	listener, err := quic.Listen(client.udpConn, generateTlsConfig(), generateQuicConfig()) // TODO
	if err != nil {
		return errors.New("cannot listen on UDP socket for incoming connectiongs.")
	}

	go client.checkin()
	go client.listenPeers(listener)

	return nil
}

func (client *Client) Forward(localAddr string, target string, targetForwardAddr string) (*forward, error) {
	log.Printf("Adding forward from local address %s to %s %s\n", localAddr, target, targetForwardAddr)

	err := client.connectToServer()
	if err != nil {
		return nil, errors.New("cannot connect to broker: " + err.Error())
	}

	// Create forward entry
	forward := &forward{
		id: client.createRandomString(8),
		status: "created", // TODO
		source: client.config.ClientUser,
		sourceAddr: localAddr,
		target: target,
		targetForwardAddr: targetForwardAddr,
	}

	client.forwardsMutex.Lock()
	defer client.forwardsMutex.Unlock()

	client.forwards[forward.id] = forward

	// Listen to local TCP address
	log.Printf("Listening on local TCP address %s\n", localAddr)
	localTcpListener, err := net.Listen("tcp", localAddr)
	if err != nil {
		return nil, err
	}

	go client.listenTcp(forward, localTcpListener)

	// Sending forward request
	log.Printf("Requesting connection to target %s on TCP address %s\n", target, targetForwardAddr)
	err = client.brokerMessenger.send(messageTypeForwardRequest, &ForwardRequest{
		Id:                forward.id,
		Source:            forward.source,
		Target:            forward.target,
		TargetForwardAddr: forward.targetForwardAddr,
	})
	if err != nil {
		return nil, err
	}

	return forward, nil
}

func (client *Client) createRandomString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func (client *Client) connectToServer() error {
	client.brokerMutex.Lock()
	defer client.brokerMutex.Unlock()

	var err error

	// Check if already connected
	if client.brokerMessenger != nil {
		return nil
	}

	// Listen to local UDP address
	udpAddr := fmt.Sprintf(":%d", 10000+rand.Intn(10000))
	log.Printf("Listening on UDP address %s\n", udpAddr)

	client.udpConn, err = net.ListenPacket("udp", udpAddr)
	if err != nil {
		return err
	}

	log.Printf("Connecting to broker at %s\n", client.brokerAddr.String())
	session, err := quic.Dial(client.udpConn, client.brokerAddr, client.brokerAddr.String(),
		generateQuicTlsClientConfig(), generateQuicConfig()) // TODO fix this
	if err != nil {
		return err
	}

	brokerStream, err := session.OpenStream()
	if err != nil {
		return err
	}

	client.brokerMessenger = &messenger{stream: brokerStream}
	go client.listenBrokerMessages()

	return nil
}

func (client *Client) listenBrokerMessages() {
	for {
		messageType, message, err := client.brokerMessenger.receive()
		if err != nil {
			if quicerr, ok := err.(*qerr.QuicError); ok && quicerr.ErrorCode == qerr.NetworkIdleTimeout {
				log.Println("Network idle timeout. Closing stream: " + err.Error())
				client.brokerMessenger.close() // TODO This should re-open the stream!
				break
			}

			log.Println("Cannot read message: " + err.Error())
			continue
		}

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

func (client *Client) listenTcp(forward *forward, listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Accepting TCP connection failed: " + err.Error())
			continue
		}

		go client.openPeerStream(forward, conn)
	}
}

func (client *Client) openPeerStream(forward *forward, conn net.Conn) {
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
		session, err := quic.Dial(client.udpConn, peerUdpAddr, sniHost, generateQuicTlsClientConfig(),
			generateQuicConfig()) // TODO fix this

		if err != nil {
			log.Println("Cannot connect to remote peer via " + peerUdpAddr.String() + ". Closing.")
			break // TODO close forward
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

	go func() { io.Copy(peerStream, conn) }()
	go func() { io.Copy(conn, peerStream) }()
}

func (client *Client) handleRegisterResponse(response *RegisterResponse) {
	// Nothing.
}

func (client *Client) handleForwardRequest(request *ForwardRequest) {
	// TODO ignore if not in "daemon mode"

	log.Printf("Accepted forward request from %s to TCP addr %s", request.Source, request.TargetForwardAddr)

	peerUdpAddr, err := net.ResolveUDPAddr("udp4", request.SourceAddr)
	if err != nil {
		log.Println("Cannot resolve peer udp addr: " + err.Error())
		client.brokerMessenger.send(messageTypeForwardResponse, &ForwardResponse{Success: false})
		return // TODO close forward
	}

	forward := &forward{
		id: request.Id,
		source: request.Source,
		sourceAddr: request.SourceAddr,
		target: request.Target,
		targetForwardAddr: request.TargetForwardAddr,
		peerUdpAddr: peerUdpAddr,
	}

	client.forwardsMutex.Lock()
	defer client.forwardsMutex.Unlock()

	client.forwards[request.Id] = forward

	err = client.brokerMessenger.send(messageTypeForwardResponse, &ForwardResponse{
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

	go client.punch(peerUdpAddr)
}

func (client *Client) handleForwardResponse(response *ForwardResponse) {
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
		forward.status = "error " + err.Error() // TODO fix me
		return
	}

	forward.status = "success"

	go client.punch(forward.peerUdpAddr)
}

func (client *Client) punch(udpAddr *net.UDPAddr) {
	// TODO add doneChan support!!

	for {
		client.udpConn.WriteTo([]byte("punch!"), udpAddr)
		time.Sleep(15 * time.Second)
	}
}

func (client *Client) checkin() {
	// TODO add doneChan support

	for {
		client.brokerMessenger.send(messageTypeRegisterRequest, &RegisterRequest{Source: client.config.ClientUser})
		time.Sleep(15 * time.Second)
	}
}

func (client *Client) handlePeerSession(session quic.Session) {
	log.Println("Session from " + session.RemoteAddr().String() + " accepted.")
	peerAddr := session.RemoteAddr().(*net.UDPAddr)
	connectionId := session.ConnectionState().ServerName // Connection ID is the SNI host!

	client.forwardsMutex.Lock()

	forward, ok := client.forwards[connectionId]
	if !ok {
		log.Printf("Cannot find forward for connection ID %s. Closing.", connectionId)
		session.Close()
		client.forwardsMutex.Unlock()
		return
	}

	targetForwardAddr := forward.targetForwardAddr
	log.Println("Client accepted from " + peerAddr.String() + ", forward found to " + targetForwardAddr)
	client.forwardsMutex.Unlock()

	for {
		stream, err := session.AcceptStream()
		if err != nil {
			log.Println("Failed to accept peer stream. Closing session: " + err.Error())
			session.Close()
			break
		}

		go client.handlePeerStream(session, stream, targetForwardAddr)
	}
}

func (client *Client) listenPeers(listener quic.Listener) {
	for {
		log.Println("Waiting for connections")
		session, err := listener.Accept()
		if err != nil {
			log.Println("Cannot accept peer connections: " + err.Error())
			time.Sleep(5 * time.Second)
			continue
		}

		go client.handlePeerSession(session)
	}
}

func (client *Client) handlePeerStream(session quic.Session, stream quic.Stream, targetForwardAddr string) {
	log.Printf("Stream %d accepted. Starting to forward.\n", stream.StreamID())

	forwardConn, err := net.Dial("tcp", targetForwardAddr)
	if err != nil {
		log.Printf("Cannot open connection to %s: %s\n", targetForwardAddr, err.Error())
		return // TODO close forward
	}

	go func() { io.Copy(stream, forwardConn) }()
	go func() { io.Copy(forwardConn, stream) }()
}
