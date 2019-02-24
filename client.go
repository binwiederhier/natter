package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"
)
func main() {
	if len(os.Args) < 4 {
		log.Fatalln("Syntax: client HUB SOURCE TARGET")

	}

	hub := os.Args[1]
	sourceName := os.Args[2]
	targetName := os.Args[3]

	hubAddr, err := net.ResolveUDPAddr("udp", hub)
	if err != nil {
		panic(err)
	}

	conn, err := net.DialUDP("udp", nil, hubAddr)
	if err != nil {
		panic(err)
	}

	var targetAddr net.Addr
	connected := false

	for !connected {
		log.Printf("Requesting public address for %s via %s ...\n", targetName, hubAddr.String())
		conn.Write([]byte(fmt.Sprintf("%s\nconnect\n%s\n", sourceName, targetName)));

		buffer := make([]byte, 1024)
		n, _, err := conn.ReadFrom(buffer)
		if err != nil {
			panic(err)
		}

		data := string(buffer[:n])
		lines := strings.Split(data, "\n")

		if lines[0] == "ok" {
			targetAddr, err = net.ResolveUDPAddr("udp", lines[1])
			if err != nil {
				panic(err)
			}
			log.Printf("Target address is %s\n", targetAddr.String())
			connected = true
		} else {
			log.Printf("Hub doesn't know %s\n", targetName)
		}

		time.Sleep(1 * time.Second)
	}




	conn.Close()
}
