package natter

import (
	"errors"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/lucas-clemente/quic-go"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type client struct {
	config        *ClientConfig
	conn          *clientConn
	forwards      map[string]*forward
	forwardsMutex sync.RWMutex
}

type forward struct {
	peerUdpAddr       *net.UDPAddr
	id                string
	source            string
	sourceAddr        string
	target            string
	targetForwardAddr string
	targetCommand     []string

	sync.RWMutex
}

func (forward *forward) PeerUdpAddr() *net.UDPAddr {
	forward.RLock()
	defer forward.RUnlock()
	return forward.peerUdpAddr
}

const (
	letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	punchInterval = 15 * time.Second
	connectionIdLength = 8
)

// NewClient creates a new client struct. It checks the configuration
// passed and returns an error if it is invalid.
func NewClient(config *ClientConfig) (Client, error) {
	rand.Seed(time.Now().UTC().UnixNano())

	if config.ClientUser == "" {
		return nil, errors.New("invalid config: ClientUser cannot be empty")
	}

	if config.BrokerAddr == "" {
		return nil, errors.New("invalid config: ServerAddr cannot be empty")
	}

	client := &client{}

	client.config = config
	client.forwards = make(map[string]*forward)

	conn, err := newClientConn(config, client.handleBrokerMessage, client.handleConnError)
	if err != nil {
		return nil, err
	}
	client.conn = conn

	return client, nil
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

func (client *client) Listen() error {
	err := client.conn.connect()
	if err != nil {
		return errors.New("cannot connect to broker: " + err.Error())
	}

	listener, err := quic.Listen(client.conn.UdpConn(), generateTlsConfig(), generateQuicConfig()) // TODO
	if err != nil {
		return errors.New("cannot listen on UDP socket for incoming connections")
	}

	go client.handleIncomingPeers(listener)

	return nil
}

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
	err = client.conn.Send(messageTypeForwardRequest, &ForwardRequest{
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


func (client *client) handleBrokerMessage(messageType messageType, message proto.Message) {
	switch messageType {
	case messageTypeCheckinResponse:
		// Ignore
	case messageTypeForwardRequest:
		client.handleForwardRequest(message.(*ForwardRequest))
	case messageTypeForwardResponse:
		client.handleForwardResponse(message.(*ForwardResponse))
	default:
		log.Println("Unknown message type", int(messageType))
	}
}

func (client *client) handleConnError() {
	log.Println("Connection error.")
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

func (client *client) handleForwardRequest(request *ForwardRequest) {
	// TODO ignore if not in "daemon mode"

	log.Printf("Accepted forward request from %s to TCP addr %s", request.Source, request.TargetForwardAddr)

	peerUdpAddr, err := net.ResolveUDPAddr("udp4", request.SourceAddr)
	if err != nil {
		log.Println("Cannot resolve peer udp addr: " + err.Error())
		client.conn.Send(messageTypeForwardResponse, &ForwardResponse{Success: false})
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

	client.forwardsMutex.Lock()
	defer client.forwardsMutex.Unlock()

	client.forwards[request.Id] = forward

	err = client.conn.Send(messageTypeForwardResponse, &ForwardResponse{
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

func (client *client) handleForwardResponse(response *ForwardResponse) {
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

func (client *client) handlePeerSession(session quic.Session) {
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
	targetCommand := forward.targetCommand

	if targetCommand != nil && len(targetCommand) > 0 {
		log.Println("Client accepted from " + peerAddr.String() + ", forward found to command " + strings.Join(forward.targetCommand, " "))
	} else {
		log.Println("Client accepted from " + peerAddr.String() + ", forward found to " + targetForwardAddr)
	}

	client.forwardsMutex.Unlock()

	for {
		stream, err := session.AcceptStream()
		if err != nil {
			log.Println("Failed to accept peer stream. Closing session: " + err.Error())
			session.Close()
			break
		}

		go client.handlePeerStream(session, stream, targetForwardAddr, targetCommand)
	}
}

func (client *client) handleIncomingPeers(listener quic.Listener) {
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

func (client *client) handlePeerStream(session quic.Session, stream quic.Stream, targetForwardAddr string, targetCommand []string) {
	log.Printf("Stream %d accepted. Starting to forward.\n", stream.StreamID())

	if targetCommand != nil && len(targetCommand) > 0 {
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
	} else {
		forwardStream, err := net.Dial("tcp", targetForwardAddr)
		if err != nil {
			log.Printf("Cannot open connection to %s: %s\n", targetForwardAddr, err.Error())
			return // TODO close forward
		}

		go func() { io.Copy(stream, forwardStream) }()
		go func() { io.Copy(forwardStream, stream) }()
	}
}

