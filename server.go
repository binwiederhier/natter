package natter

import (
	"fmt"
	"github.com/lucas-clemente/quic-go"
	"log"
	"math/rand"
	"net"
)

type conn struct {
	addr   *net.UDPAddr
	stream quic.Stream
}

type fwd struct {
	id         string
	source     string
	sourceConn *conn
	target     string
	targetConn *conn
}

func NewServer() *server {
	return &server{}
}

type server struct {
	control  map[string]*conn
	forwards map[string]*fwd
}

func (s *server) Start(listenAddr string) {
	s.control = make(map[string]*conn)
	s.forwards = make(map[string]*fwd)

	listener, err := quic.ListenAddr(listenAddr, generateTLSConfig(), generateQuicConfig()) // TODO fix this
	if err != nil {
		panic(err)
	}

	log.Println("[server] Waiting for connections")
	for {
		session, err := listener.Accept()
		if err != nil {
			panic(err)
		}

		go s.handleSession(session)
	}
}

func (s *server) handleSession(session quic.Session) {
	for {
		stream, err := session.AcceptStream()
		if err != nil {
			log.Println("[server] Session err: " + err.Error())
			session.Close()
			return
		}

		go s.handleStream(session, stream)
	}
}

func (s *server) handleStream(session quic.Session, stream quic.Stream) {
	addr := session.RemoteAddr()

	for {
		messageType, message := receiveMessage(stream)
		udpAddr, _ := addr.(*net.UDPAddr)

		switch messageType {
		case messageTypeRegisterRequest:
			request, _ := message.(*RegisterRequest)
			remoteAddr := fmt.Sprintf("%s:%d", udpAddr.IP, udpAddr.Port)

			log.Println("[server] Client", request.Source, "with address", remoteAddr, "connected")

			s.control[request.Source] = &conn{
				stream: stream,
				addr:   udpAddr,
			}

			log.Println("[server] Control table:")
			for client, conn := range s.control {
				log.Println("[server] -", client, conn.addr)
			}

			sendMessage(stream, messageTypeRegisterResponse, &RegisterResponse{Addr: remoteAddr})
		case messageTypeForwardRequest:
			request, _ := message.(*ForwardRequest)

			if _, ok := s.control[request.Target]; !ok {
				sendMessage(stream, messageTypeForwardResponse, &ForwardResponse{Success: false})
			} else {
				targetControl := s.control[request.Target]


				forward := &fwd{
					id:         request.Id,
					source:     request.Source,
					sourceConn: &conn{addr: udpAddr, stream: stream},
					target:     request.Target,
					targetConn: nil,
				}

				log.Printf("[server] Adding new connection %s\n", request.Id)
				s.forwards[request.Id] = forward

				sendMessage(targetControl.stream, messageTypeForwardRequest, &ForwardRequest{
					Id:                request.Id,
					Source:            request.Source,
					SourceAddr:        fmt.Sprintf("%s:%d", udpAddr.IP, udpAddr.Port),
					Target:            request.Target,
					TargetAddr:        fmt.Sprintf("%s:%d", targetControl.addr.IP, targetControl.addr.Port),
					TargetForwardAddr: request.TargetForwardAddr,
				})
			}
		case messageTypeForwardResponse:
			response, _ := message.(*ForwardResponse)

			if _, ok := s.forwards[response.Id]; !ok {
				log.Println("[server] Cannot forward response")
			} else {
				fwd := s.forwards[response.Id]
				sendMessage(fwd.sourceConn.stream, messageTypeForwardResponse, response)
			}
		}
	}
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func createRandomString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
