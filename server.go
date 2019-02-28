package main

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"log"
	"net"
)

type server struct {
	userIP map[string]string
}

func (s *server) start(listenAddr string) {
	s.userIP = map[string]string{}
	udpAddr, err := net.ResolveUDPAddr("udp4", listenAddr)
	if err != nil {
		log.Fatal(err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatal(err)
	}

	for {
		s.handleClient(conn)
	}
}

func (s *server) handleClient(conn *net.UDPConn) {
	var buf [2048]byte

	n, addr, err := conn.ReadFromUDP(buf[0:])
	if err != nil {
		return
	}

	fmt.Println("<- ", string(buf[:n]))

	messageType := messageType(buf[0])

	switch messageType {
	case messageTypeRegisterRequest:
		var request RegisterRequest
		err = proto.Unmarshal(buf[1:n], &request)
		if err != nil {
			log.Print(err)
			return
		}
		remoteAddr := fmt.Sprintf("%s:%d", addr.IP, addr.Port)
		fmt.Println(request.Source, remoteAddr, "connecting")
		s.userIP[request.Source] = remoteAddr

		sendmsg(conn, addr, messageTypeRegisterResponse, &RegisterResponse{Addr: remoteAddr})
	case messageTypeGetRequest:
		var request GetRequest
		err = proto.Unmarshal(buf[1:n], &request)
		if err != nil {
			log.Print(err)
			return
		}

		// Send message back
		targetAddr := ""
		if _, ok := s.userIP[request.Target]; ok {
			targetAddr = s.userIP[request.Target]
		}

		response := &GetResponse{TargetAddr: targetAddr}

		log.Printf("<- GetRequest %s\n", request.String())
		log.Printf("<- GetResponse %s\n", response.String())

		sendmsg(conn, addr, messageTypeGetResponse, response)
	}

	fmt.Println("User table:", s.userIP)
}

