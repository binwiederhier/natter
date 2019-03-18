package natter

import (
	"errors"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/lucas-clemente/quic-go"
	"heckel.io/natter/internal"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"
)

const (
	checkLoopSleep = 15 * time.Second
)

type messageCallback func (messageType messageType, message proto.Message)
type errorCallback func ()

type clientConn struct {
	config          *ClientConfig
	messageCallback messageCallback
	errorCallback   errorCallback

	session       quic.Session
	proto         *protocol
	udpBrokerAddr *net.UDPAddr
	udpConn       net.PacketConn

	exitChan      chan int
	connectedChan chan int

	disconnectOnce sync.Once
	mutex          sync.RWMutex
}

func newClientConn(config *ClientConfig, messageCallback messageCallback, errorCallback errorCallback) (*clientConn, error) {
	udpBrokerAddr, err := net.ResolveUDPAddr("udp4", config.BrokerAddr)
	if err != nil {
		return nil, err
	}

	udpConn, err := net.ListenPacket("udp", fmt.Sprintf(":%d", 10000+rand.Intn(10000)))
	if err != nil {
		return nil, err
	}

	return &clientConn{
		config:          config,
		udpBrokerAddr:   udpBrokerAddr,
		udpConn:         udpConn,
		messageCallback: messageCallback,
		errorCallback:   errorCallback,
	}, nil
}

func (b *clientConn) connect() error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	var err error

	// Check if already connected
	if b.proto != nil {
		return nil
	}

	log.Printf("Connecting to broker at %s\n", b.udpBrokerAddr.String())
	b.session, err = quic.Dial(b.udpConn, b.udpBrokerAddr, b.udpBrokerAddr.String(),
		generateQuicTlsClientConfig(), generateQuicConfig()) // TODO fix this
	if err != nil {
		return err
	}

	stream, err := b.session.OpenStream()
	if err != nil {
		return err
	}

	b.proto = &protocol{stream: stream}
	b.exitChan = make(chan int)
	b.connectedChan = make(chan int)

	go b.handleIncoming()
	go b.handleCheckinLoop()

	select {
	case <- b.connectedChan:
		return nil
	case <- time.After(5 * time.Second):
		return errors.New("timed out while connecting")
	}
}

func (b *clientConn) disconnect() {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	// Check if connected
	if b.proto == nil {
		return
	}

	// Run showdown method
	b.disconnectOnce.Do(func() {
		log.Println("Shutting down broker connection ...")

		close(b.exitChan)
		close(b.connectedChan)

		b.proto.close()
		b.session.Close()

		b.proto = nil
		b.session = nil

		b.errorCallback()
	})
}

func (b *clientConn) Send(messageType messageType, message proto.Message) error {
	return b.proto.send(messageType, message)
}

func (b *clientConn) UdpConn() net.PacketConn {
	return b.udpConn
}

func (b *clientConn) handleIncoming() {
	defer func() {
		log.Println("Exiting read loop due to error")
		b.disconnect()
	}()

	var connected bool

	for {
		select {
		case <- b.exitChan:
			return
		default:
		}

		messageType, message, err := b.proto.receive()
		if err != nil {
			log.Println("Error reading message from broker connection: " + err.Error())
			return
		}

		switch messageType {
		case messageTypeCheckinResponse:
			if !connected {
				log.Println("Successfully connected to broker")
				connected = true
				b.connectedChan <- 1
			}
		}

		b.messageCallback(messageType, message)
	}
}

func (b *clientConn) handleCheckinLoop() {
	defer func() {
		log.Println("Exiting checkin loop due to error")
		b.disconnect()
	}()

	for {
		err := b.proto.send(messageTypeCheckinRequest, &internal.CheckinRequest{Source: b.config.ClientUser})
		if err != nil {
			log.Println("Error sending checking request to broker: " + err.Error())
			return
		}

		select {
		case <- b.exitChan:
			return
		case <- time.After(checkLoopSleep):
		}
	}
}
