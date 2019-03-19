package natter

import (
	"crypto/tls"
	"errors"
	"github.com/golang/protobuf/proto"
	"github.com/lucas-clemente/quic-go"
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
	letterBytes                = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	punchInterval              = 15 * time.Second
	connectionIdLength         = 8
	connectionIdleTimeout      = 5 * time.Second
	connectionHandshakeTimeout = 5 * time.Second
)

// NewClient creates a new client struct. It checks the configuration
// passed and returns an error if it is invalid.
func NewClient(config *Config) (Client, error) {
	rand.Seed(time.Now().UTC().UnixNano())

	config, err := populateClientConfig(config)
	if err != nil {
		return nil, err
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
	if config.ClientUser == "" {
		return nil, errors.New("invalid config: ClientUser cannot be empty")
	}

	if config.BrokerAddr == "" {
		return nil, errors.New("invalid config: ServerAddr cannot be empty")
	}

	newConfig := &Config{
		ClientUser: config.ClientUser,
		BrokerAddr: config.BrokerAddr,
	}

	if config.QuicConfig == nil {
		newConfig.QuicConfig = generateDefaultQuicConfig()
	}

	if config.TLSConfig == nil {
		newConfig.TLSConfig = generateDefaultTLSClientConfig()
	}

	return newConfig, nil
}

func generateDefaultQuicConfig() *quic.Config {
	return &quic.Config{
		KeepAlive:          true,
		Versions:           []quic.VersionNumber{quic.VersionGQUIC43}, // Version 44 does not support multiplexing!
		ConnectionIDLength: connectionIdLength,
		IdleTimeout:        connectionIdleTimeout,
		HandshakeTimeout:   connectionHandshakeTimeout,
	}
}

func generateDefaultTLSClientConfig() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: true,
	}
}
