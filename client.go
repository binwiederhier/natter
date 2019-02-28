package main

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"log"
	"math/rand"
	"net"
	"time"
)

type client struct {
	sourceUdpServerAddr *net.UDPAddr
	sourceUdpConnection *net.UDPConn
}

func (c *client) start(hubAddr string, source string, sourcePort int, target string, targetPort int) {
	var err error
	buf := make([]byte, 2048)

	rand.Seed(time.Now().Unix())
	sourceUdpServerPort := fmt.Sprintf(":%d", 10000 + rand.Intn(10000))

	// Prepare to register user to server.
	c.sourceUdpServerAddr, err = net.ResolveUDPAddr("udp4", hubAddr)
	if err != nil {
		log.Print("Resolve server address failed.")
		log.Fatal(err)
	}

	// Prepare for local listening.
	addr, err := net.ResolveUDPAddr("udp4", sourceUdpServerPort)
	if err != nil {
		log.Print("Resolve local address failed.")
		log.Fatal(err)
	}

	c.sourceUdpConnection, err = net.ListenUDP("udp", addr)
	if err != nil {
		log.Print("Listen UDP failed.")
		log.Fatal(err)
	}

	// Send registration information to server.
	sendmsg(c.sourceUdpConnection, c.sourceUdpServerAddr, messageTypeRegisterRequest,
		&RegisterRequest{Source: source})

	log.Print("Waiting for server response...")
	n, _, err := c.sourceUdpConnection.ReadFromUDP(buf)
	if err != nil {
		log.Print("Register to server failed.")
		log.Fatal(err)
	}
	fmt.Println("<- (unused) ", string(buf[:n]))

	// Send connect request to server

	var response GetResponse

	for i := 0; i < 3; i++ {
		sendmsg(c.sourceUdpConnection, c.sourceUdpServerAddr, messageTypeGetRequest,
			&GetRequest{Source: source, Target: target})

		n, _, err := c.sourceUdpConnection.ReadFromUDP(buf)
		if err != nil {
			log.Print("Get peer address from server failed.")
			log.Fatal(err)
		}
		fmt.Println("<- ", string(buf[:n]))

		messageType := messageType(buf[0])

		switch messageType {
		case messageTypeGetResponse:
			err = proto.Unmarshal(buf[1:n], &response)
			if err != nil {
				panic(err)
			}

			break
		}

		time.Sleep(10 * time.Second)
	}

	if response.TargetAddr == "" {
		log.Fatal("Cannot get peer's address")
	}

	log.Print("Peer address: ", response.TargetAddr)
	peerAddr, err := net.ResolveUDPAddr("udp4", response.TargetAddr)
	if err != nil {
		log.Print("Resolve peer address failed.")
		log.Fatal(err)
	}

	// Start chatting.
	go c.listen(c.sourceUdpConnection)
	for {
		fmt.Print("Input message: ")
		message := make([]byte, 2048)
		n, err := fmt.Scanln(&message)
		msgstr := ""

		if n > 0 && err == nil {
			msgstr = string(message)
		}

		sendmsg(c.sourceUdpConnection, peerAddr, messageTypeChatMessage,
			&ChatMessage{Message: msgstr})
	}
}

func (c *client) listen(conn *net.UDPConn) {
	for {
		buf := make([]byte, 2048)
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Print(err)
			continue
		}
		// log.Print("Message from ", addr.IP)
		fmt.Println("<- ", string(buf[:n]))

		messageType := messageType(buf[0])

		switch messageType {
		case messageTypeChatMessage:
			var msg ChatMessage
			err = proto.Unmarshal(buf[1:n], &msg)
			if err != nil {
				panic(err)
			}

			fmt.Println(msg.Message)
		}
	}
}
