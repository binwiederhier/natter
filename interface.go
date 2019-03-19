package natter

import (
	"crypto/tls"
	"github.com/lucas-clemente/quic-go"
	"net"
)

type Forward interface {
	PeerUdpAddr() *net.UDPAddr
}

// Client
type Client interface {
	Listen() error
	Forward(localAddr string, target string, targetForwardAddr string, targetCommand []string) (Forward, error)
}

// Config defines the configuration for a natter client.
type Config struct {
	// Identifier used to uniquely identify individual clients.
	// It is important and required to be able to connect to other clients.
	// Example: myclient123
	ClientUser string

	// Hostname and port of the broker. The broker is only used to connect
	// the two peers. Example: heckel.io:2568
	BrokerAddr string

	// Configure TLS between client and server
	TLSConfig *tls.Config

	// Override the QUIC configuration. This should not be necessary and
	// can break the functionality, if not done correctly. Ideally, you should
	// not change any settings here.
	QuicConfig *quic.Config
}
