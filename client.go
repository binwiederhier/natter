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
	config        *Config
	conn          *clientConn
	forwards      map[string]*forward
	forwardsMutex sync.RWMutex
}

type forward struct {
	peerUdpAddr       *net.UDPAddr
	id                string
	mode              string
	source            string
	sourceAddr        string
	sourceNetwork     string
	target            string
	targetForwardAddr string
	targetNetwork     string
	targetCommand     []string

	sync.RWMutex
}

func (forward *forward) PeerUdpAddr() *net.UDPAddr {
	forward.RLock()
	defer forward.RUnlock()
	return forward.peerUdpAddr
}

const (
	letterBytes                = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	punchInterval              = 15 * time.Second
	connectionIdLength         = 8
	connectionIdleTimeout      = 5 * time.Second
	connectionHandshakeTimeout = 5 * time.Second
)

const (
	forwardTypeTCP = "TCP"
	forwardTypeL2 = "L2"
)

// NewClient creates a new client struct. It checks the configuration
// passed and returns an error if it is invalid.
func NewClient(config *Config) (Client, error) {
	rand.Seed(time.Now().UTC().UnixNano())

	newConfig, err := populateClientConfig(config)
	if err != nil {
		return nil, err
	}

	client := &client{}

	client.config = newConfig
	client.forwards = make(map[string]*forward)

	conn, err := newClientConn(newConfig, client.handleBrokerMessage, client.handleConnError)
	if err != nil {
		return nil, err
	}
	client.conn = conn

	return client, nil
}

func (c *client) handleBrokerMessage(messageType messageType, message proto.Message) {
	switch messageType {
	case messageTypeCheckinResponse:
		// Ignore
	case messageTypeForwardRequest:
		c.handleForwardRequest(message.(*internal.ForwardRequest))
	case messageTypeForwardResponse:
		c.handleForwardResponse(message.(*internal.ForwardResponse))
	default:
		log.Println("Unknown message type", int(messageType))
	}
}

func (c *client) handleConnError() {
	log.Println("Connection error.")
}

func (c *client) punch(udpAddr *net.UDPAddr) {
	// TODO add exitChan support!!

	for {
		udpConn := c.conn.UdpConn()

		if udpConn != nil {
			udpConn.WriteTo([]byte("punch!"), udpAddr)
		}

		time.Sleep(punchInterval)
	}
}

func populateClientConfig(config *Config) (*Config, error) {
	if config.ClientId == "" {
		return nil, errors.New("invalid config: ClientId cannot be empty")
	}

	if config.BrokerAddr == "" {
		return nil, errors.New("invalid config: ServerAddr cannot be empty")
	}

	newConfig := &Config{
		ClientId:   config.ClientId,
		BrokerAddr: config.BrokerAddr,
	}

	if config.QuicConfig == nil {
		newConfig.QuicConfig = generateDefaultQuicConfig()
	}

	if config.TLSServerConfig == nil {
		newConfig.TLSServerConfig = generateDefaultTLSServerConfig()
	}

	if config.TLSClientConfig == nil {
		newConfig.TLSClientConfig = generateDefaultTLSClientConfig()
	}

	return newConfig, nil
}
