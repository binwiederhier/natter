package natter

import (
	"errors"
	"fmt"
	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/qerr"
	"heckel.io/natter/internal"
	"log"
	"net"
	"sync"
)

type broker struct {
	config   *Config
	clients  map[string]*brokerClient
	forwards map[string]*brokerForward

	mutex sync.RWMutex
}

type brokerClient struct {
	session quic.Session
	proto   *protocol
	addr    *net.UDPAddr
}

type brokerForward struct {
	source *brokerClient
	target *brokerClient
}

func NewBroker(config *Config) (Broker, error) {
	newConfig, err := populateBrokerConfig(config)
	if err != nil {
		return nil, err
	}

	return &broker{
		config: newConfig,
		clients: make(map[string]*brokerClient),
		forwards: make(map[string]*brokerForward),
	}, nil
}

func (b *broker) ListenAndServe() error {
	tlsServerConfig := *b.config.TLSServerConfig // copy, because quic-go alters it!
	listener, err := quic.ListenAddr(b.config.BrokerAddr, &tlsServerConfig, b.config.QuicConfig)
	if err != nil {
		return err
	}

	log.Println("Waiting for connections")

	for {
		session, err := listener.Accept()
		if err != nil {
			log.Println("Accepting client failed: " + err.Error())
			continue
		}

		go b.handleSession(session)
	}

	return nil
}

func (b *broker) handleSession(session quic.Session) {
	for {
		stream, err := session.AcceptStream()
		if err != nil {
			log.Println("Session err: " + err.Error())
			session.Close()
			return
		}

		client := &brokerClient{
			addr:    session.RemoteAddr().(*net.UDPAddr),
			session: session,
			proto:   &protocol{stream: stream},
		}

		go b.handleClient(client)
	}
}

func (b *broker) handleClient(client *brokerClient) {
	for {
		messageType, message, err := client.proto.receive()

		if err != nil {
			if quicerr, ok := err.(*qerr.QuicError); ok && quicerr.ErrorCode == qerr.NetworkIdleTimeout {
				log.Println("Network idle timeout. Closing stream: " + err.Error())
				client.session.Close()
				break
			}


			log.Println("Cannot read message: " + err.Error() + ". Closing session.")
			client.session.Close()
			break
		}

		switch messageType {
		case messageTypeCheckinRequest:
			b.handleCheckinRequest(client, message.(*internal.CheckinRequest))
		case messageTypeForwardRequest:
			b.handleForwardRequest(client, message.(*internal.ForwardRequest))
		case messageTypeForwardResponse:
			b.handleForwardResponse(client, message.(*internal.ForwardResponse))
		}
	}
}

func (b *broker) handleCheckinRequest(client *brokerClient, request *internal.CheckinRequest) {
	remoteAddr := fmt.Sprintf("%s:%d", client.addr.IP, client.addr.Port)

	log.Println("Client", request.Source, "with address", remoteAddr, "connected")

	b.mutex.Lock()
	b.clients[request.Source] = client
	b.mutex.Unlock()

	log.Println("Control table:")
	for client, conn := range b.clients {
		log.Println("-", client, conn.addr)
	}

	err := client.proto.send(messageTypeCheckinResponse, &internal.CheckinResponse{Addr: remoteAddr})
	if err != nil {
		log.Println("Cannot respond to client: " + err.Error())
	}
}

func (b *broker) handleForwardRequest(client *brokerClient, request *internal.ForwardRequest) {
	b.mutex.RLock()
	target , ok := b.clients[request.Target]
	b.mutex.RUnlock()

	if !ok {
		err := client.proto.send(messageTypeForwardResponse, &internal.ForwardResponse{
			Id:      request.Id,
			Success: false,
		})
		if err != nil {
			log.Printf("Failed to respond to forward request: " + err.Error())
		}
	} else {
		forward := &brokerForward{
			source: client,
			target: nil,
		}

		log.Printf("Adding new connection %s\n", request.Id)

		b.mutex.Lock()
		b.forwards[request.Id] = forward
		b.mutex.Unlock()

		request.SourceAddr = fmt.Sprintf("%s:%d", client.addr.IP, client.addr.Port)
		request.TargetAddr = fmt.Sprintf("%s:%d", target.addr.IP, target.addr.Port)

		err := target.proto.send(messageTypeForwardRequest, request)
		if err != nil {
			log.Printf("Failed to respond to forward request: " + err.Error())
		}
	}
}

func (b *broker) handleForwardResponse(client *brokerClient, response *internal.ForwardResponse) {
	b.mutex.RLock()
	forward, ok := b.forwards[response.Id]
	b.mutex.RUnlock()

	if !ok {
		log.Println("Cannot forward response, cannot find connection")
	} else {
		err := forward.source.proto.send(messageTypeForwardResponse, response)
		if err != nil {
			log.Printf("Failed to forward to forward response: " + err.Error())
		}
	}
}

func populateBrokerConfig(config *Config) (*Config, error) {
	if config.BrokerAddr == "" {
		return nil, errors.New("invalid config: BrokerAddr cannot be empty")
	}

	newConfig := &Config{
		BrokerAddr: config.BrokerAddr,
	}

	if config.QuicConfig == nil {
		newConfig.QuicConfig = generateDefaultQuicConfig()
	}

	if config.TLSServerConfig == nil {
		newConfig.TLSServerConfig = generateDefaultTLSServerConfig()
	}

	return newConfig, nil
}
