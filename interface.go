package natter

import (
	"crypto/tls"
	"github.com/lucas-clemente/quic-go"
	"net"
)

// Client represents a natter client. It can be used to listen for
// incoming connections and/or to open forwards to other clients.
type Client interface {
	// Listen listens on a UDP/QUIC connection for incoming peer connections.
	Listen() error

	// Forward requests a forward connection to another client.
	//
	// localAddr is the local TCP [address]:port that shall be forwarded, e.g. 10.0.10.1:9000
	// If the address is omitted, all local addressed will be bound to, e.g. :9000
	// If it is empty, STDIN is read.
	//
	// target is the client identifier (see ClientId below) of the target client.
	// It cannot be empty.
	//
	// targetForwardAddr is the target TCP [address]:port on the target client, e.g. 192.168.1.2:22
	// If the address is omitted, localhost is used, e.g. :22 is equivalent to 127.0.0.1:22
	// If the address is a non-local address, traffic is forwarded to another host via the target machine,
	// e.g. google.com:80 will forward to Google's web server
	//
	// targetCommand can be used to execute a command on the target host and forward its STDIN,
	// e.g. []string { "zfs", "recv" } or []string{ "sh", "-c", "cat > hello.txt" }.
	// If targetCommand is set, targetForwardAddr is ignored.
	ForwardTCP(localAddr string, target string, targetForwardAddr string, targetCommand []string) (Forward, error)

	ForwardL2(localNetwork string, target string, targetNetwork string) (Forward, error)
}

type Broker interface {
	ListenAndServe() error
}

type Forward interface {
	PeerUdpAddr() *net.UDPAddr
}

// Config defines the configuration for a natter client or broker.
type Config struct {
	// Identifier used to uniquely identify individual clients. It is important
	// and required to be able to connect to other clients. Example: myclient123
	ClientId string

	// Hostname and port of the broker. The broker is only used to connect
	// the two peers. Example: heckel.io:2568
	BrokerAddr string

	// Configure TLS for all TLS clients
	TLSClientConfig *tls.Config

	// Configure TLS between client and server
	TLSServerConfig *tls.Config

	// Override the QUIC configuration. This should not be necessary and
	// can break the functionality, if not done correctly. Ideally, you should
	// not change any settings here.
	QuicConfig *quic.Config

	// TODO Add "Listen" and "Forwards" flags
}
