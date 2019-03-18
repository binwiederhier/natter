package natter

import (
	"github.com/lucas-clemente/quic-go"
	"net"
)

type Forward interface {
	PeerUdpAddr() *net.UDPAddr
}

type Client interface {
	Listen() error
	Forward(localAddr string, target string, targetForwardAddr string, targetCommand []string) (Forward, error)
}

type ClientConfig struct {
	ClientUser string
	BrokerAddr string
	QuicConfig quic.Config
}
