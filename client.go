package natter

import (
	"errors"
	"github.com/golang/protobuf/proto"
	"heckel.io/natter/internal"
	"log"
	"math/rand"
	"net"
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

func (client *client) handleBrokerMessage(messageType messageType, message proto.Message) {
	switch messageType {
	case messageTypeCheckinResponse:
		// Ignore
	case messageTypeForwardRequest:
		client.handleForwardRequest(message.(*internal.ForwardRequest))
	case messageTypeForwardResponse:
		client.handleForwardResponse(message.(*internal.ForwardResponse))
	default:
		log.Println("Unknown message type", int(messageType))
	}
}

func (client *client) handleConnError() {
	log.Println("Connection error.")
}
