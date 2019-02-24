package main

import (
	"fmt"
	"net"
	"strings"
)

func main() {
	clients := make(map[string]net.Addr)

	s, err := net.ListenPacket("udp", ":20001")

	if err != nil {
		panic(err)
	}

	buffer := make([]byte, 1024)

	for {
		n, sourceAddr, err := s.ReadFrom(buffer)

		if err != nil {
			panic(err)
		}

		data := string(buffer[:n])

		fmt.Printf("packet-received: bytes=%d from=%s\n%s\n",
			n, sourceAddr.String(), data)

		lines := strings.Split(data, "\n")

		sourceName := lines[0]
		method := lines[1]

		switch method {
		case "connect":
			targetName := lines[2]
			clients[sourceName] = sourceAddr

			if targetAddr, ok := clients[targetName]; ok {
				s.WriteTo([]byte("ok\n" + targetAddr.String() + "\n"), sourceAddr)
			} else {
				s.WriteTo([]byte("nok\n"), sourceAddr)
			}
		}

	}
}
