package natter

import (
	"fmt"
	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/qerr"
	"log"
	"net"
)

type client struct {
	addr   *net.UDPAddr
	messenger *messenger
}

type fwd struct {
	sourceConn *client
	targetConn *client
}

func NewServer() *server {
	return &server{}
}

type server struct {
	clients  map[string]*client
	forwards map[string]*fwd
}

func (s *server) Start(listenAddr string) {
	s.clients = make(map[string]*client)
	s.forwards = make(map[string]*fwd)

	listener, err := quic.ListenAddr(listenAddr, generateTlsConfig(), generateQuicConfig()) // TODO fix this
	if err != nil {
		panic(err)
	}

	log.Println("Waiting for connections")
	for {
		session, err := listener.Accept()
		if err != nil {
			log.Println("Accepting client failed: " + err.Error())
			continue
		}

		go s.handleSession(session)
	}
}

func (s *server) handleSession(session quic.Session) {
	for {
		stream, err := session.AcceptStream()
		if err != nil {
			log.Println("Session err: " + err.Error())
			session.Close()
			return
		}

		messenger := &messenger{stream: stream}
		go s.handleStream(session, messenger)
	}
}

func (s *server) handleStream(session quic.Session, messenger *messenger) {
	addr := session.RemoteAddr()

	for {
		udpAddr, _ := addr.(*net.UDPAddr)
		messageType, message, err := messenger.receive()

		if err != nil {
			if quicerr, ok := err.(*qerr.QuicError); ok && quicerr.ErrorCode == qerr.NetworkIdleTimeout {
				log.Println("Network idle timeout. Closing stream: " + err.Error())
				messenger.close()
				break
			}

			log.Println("Cannot read message: " + err.Error())
			continue
		}

		switch messageType {
		case messageTypeRegisterRequest:
			request, _ := message.(*RegisterRequest)
			remoteAddr := fmt.Sprintf("%s:%d", udpAddr.IP, udpAddr.Port)

			log.Println("Client", request.Source, "with address", remoteAddr, "connected")

			s.clients[request.Source] = &client{
				messenger: messenger,
				addr:   udpAddr,
			}

			log.Println("Control table:")
			for client, conn := range s.clients {
				log.Println("-", client, conn.addr)
			}

			messenger.send(messageTypeRegisterResponse, &RegisterResponse{Addr: remoteAddr})
		case messageTypeForwardRequest:
			request, _ := message.(*ForwardRequest)

			if _, ok := s.clients[request.Target]; !ok {
				messenger.send(messageTypeForwardResponse, &ForwardResponse{
					Id: request.Id,
					Success: false,
				})
			} else {
				target := s.clients[request.Target]

				forward := &fwd{
					sourceConn: &client{addr: udpAddr, messenger: messenger},
					targetConn: nil,
				}

				log.Printf("Adding new connection %s\n", request.Id)
				s.forwards[request.Id] = forward

				target.messenger.send(messageTypeForwardRequest, &ForwardRequest{
					Id:                request.Id,
					Source:            request.Source,
					SourceAddr:        fmt.Sprintf("%s:%d", udpAddr.IP, udpAddr.Port),
					Target:            request.Target,
					TargetAddr:        fmt.Sprintf("%s:%d", target.addr.IP, target.addr.Port),
					TargetForwardAddr: request.TargetForwardAddr,
				})
			}
		case messageTypeForwardResponse:
			response, _ := message.(*ForwardResponse)

			if _, ok := s.forwards[response.Id]; !ok {
				log.Println("Cannot forward response")
			} else {
				fwd := s.forwards[response.Id]
				source := fwd.sourceConn
				source.messenger.send(messageTypeForwardResponse, response)
			}
		}
	}
}
