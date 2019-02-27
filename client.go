package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
	"time"
)


func startClient(serverAddr string, username string, localPort int, peer string, targetPort int) {
	rand.Seed(time.Now().Unix())

	localUdpPort := fmt.Sprintf(":%d", 1000 + rand.Intn(1000))
	buf := make([]byte, 2048)

	// Prepare to register user to server.
	saddr, err := net.ResolveUDPAddr("udp4", serverAddr)
	if err != nil {
		log.Print("Resolve server address failed.")
		log.Fatal(err)
	}

	// Prepare for local listening.
	addr, err := net.ResolveUDPAddr("udp4", localUdpPort)
	if err != nil {
		log.Print("Resolve local address failed.")
		log.Fatal(err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Print("Listen UDP failed.")
		log.Fatal(err)
	}

	// Send registration information to server.
	initChatRequest := ChatRequest{
		"New",
		username,
		"",
	}
	jsonRequest, err := json.Marshal(initChatRequest)
	if err != nil {
		log.Print("Marshal Register information failed.")
		log.Fatal(err)
	}
	fmt.Println("-> ", string(jsonRequest))
	_, err = conn.WriteToUDP(jsonRequest, saddr)
	if err != nil {
		log.Fatal(err)
	}

	log.Print("Waiting for server response...")
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		log.Print("Register to server failed.")
		log.Fatal(err)
	}
	fmt.Println("<- (unused) ", string(buf[:n]))

	// Send connect request to server
	connectChatRequest := ChatRequest{
		"Get",
		username,
		peer,
	}
	jsonRequest, err = json.Marshal(connectChatRequest)
	if err != nil {
		log.Print("Marshal connection information failed.")
		log.Fatal(err)
	}

	var serverResponse ChatRequest
	for i := 0; i < 3; i++ {
		fmt.Println("-> ", string(jsonRequest))
		conn.WriteToUDP(jsonRequest, saddr)
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Print("Get peer address from server failed.")
			log.Fatal(err)
		}
		fmt.Println("<- ", string(buf[:n]))
		err = json.Unmarshal(buf[:n], &serverResponse)
		if err != nil {
			log.Print("Unmarshal server response failed.")
			log.Fatal(err)
		}
		if serverResponse.Message != "" {
			break
		}
		time.Sleep(10 * time.Second)
	}
	if serverResponse.Message == "" {
		log.Fatal("Cannot get peer's address")
	}
	log.Print("Peer address: ", serverResponse.Message)
	peerAddr, err := net.ResolveUDPAddr("udp4", serverResponse.Message)
	if err != nil {
		log.Print("Resolve peer address failed.")
		log.Fatal(err)
	}

	// Start chatting.
	go listen(conn)
	for {
		fmt.Print("Input message: ")
		message := make([]byte, 2048)
		fmt.Scanln(&message)
		messageRequest := ChatRequest{
			"Chat",
			username,
			string(message),
		}
		jsonRequest, err = json.Marshal(messageRequest)
		if err != nil {
			log.Print("Error: ", err)
			continue
		}
		fmt.Println("-> ", string(jsonRequest))
		conn.WriteToUDP(jsonRequest, peerAddr)
	}
}

func listen(conn *net.UDPConn) {
	for {
		buf := make([]byte, 2048)
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Print(err)
			continue
		}
		// log.Print("Message from ", addr.IP)
		fmt.Println("<- ", string(buf[:n]))
		var message ChatRequest
		err = json.Unmarshal(buf[:n], &message)
		if err != nil {
			log.Print(err)
			continue
		}
		fmt.Println(message.Username, ":", message.Message)
	}
}
