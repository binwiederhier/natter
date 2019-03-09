package main

import (
	"fmt"
	"github.com/lucas-clemente/quic-go"
	"log"
	"math/rand"
	"net"
)

type clientconn struct {
	addr   *net.UDPAddr
	stream quic.Stream
}

type fwd struct {
	id         string
	source     string
	sourceConn *clientconn
	target     string
	targetConn *clientconn
}

type server struct {
	control  map[string]*clientconn
	forwards map[string]*fwd
}

func (s *server) start(listenAddr string) {
	s.control = make(map[string]*clientconn)
	s.forwards = make(map[string]*fwd)

	listener, err := quic.ListenAddr(listenAddr, generateTLSConfig(), &quic.Config{
		ConnectionIDLength: 8,
		Versions:           []quic.VersionNumber{quic.VersionGQUIC43}})
	if err != nil {
		panic(err)
	}

	log.Println("Waiting for connections")
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
			log.Println("Session err: " + err.Error())
			session.Close()
			return
		}

		go s.handleStream(session, stream)
	}
}

func (s *server) handleStream(session quic.Session, stream quic.Stream) {
	addr := session.RemoteAddr()

	for {
		messageType, message := recvmsg2(stream)
		udpAddr, _ := addr.(*net.UDPAddr)

		switch messageType {
		case messageTypeRegisterRequest:
			request, _ := message.(*RegisterRequest)
			remoteAddr := fmt.Sprintf("%s:%d", udpAddr.IP, udpAddr.Port)

			log.Println("Client", request.Source, "with address", remoteAddr, "connected")

			s.control[request.Source] = &clientconn{
				stream: stream,
				addr:   udpAddr,
			}

			log.Println("Control table:")
			for client, conn := range s.control {
				log.Println("-", client, conn.addr)
			}

			sendmsg(stream, messageTypeRegisterResponse, &RegisterResponse{Addr: remoteAddr})
		case messageTypeForwardRequest:
			request, _ := message.(*ForwardRequest)

			if _, ok := s.control[request.Target]; !ok {
				sendmsg(stream, messageTypeForwardResponse, &ForwardResponse{Success: false})
			} else {
				targetControl := s.control[request.Target]

				id := createRandomString(8)
				forward := &fwd{
					id:         id,
					source:     request.Source,
					sourceConn: &clientconn{addr: udpAddr, stream: stream},
					target:     request.Target,
					targetConn: nil,
				}

				log.Printf("Adding new connection %s\n", id)
				s.forwards[id] = forward

				sendmsg(targetControl.stream, messageTypeForwardRequest, &ForwardRequest{
					Id:                id,
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
				log.Println("cannot forward response")
			} else {
				fwd := s.forwards[response.Id]
				sendmsg(fwd.sourceConn.stream, messageTypeForwardResponse, response)
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
