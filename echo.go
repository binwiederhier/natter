package main

import (
	"fmt"
	"net"
	"os"
)

func main() {
	l, err := net.ListenPacket("udp", os.Args[1])

	if err != nil {
		panic(err)
	}

	buffer := make([]byte, 2048)

	for {
		n, addr, err := l.ReadFrom(buffer)

		if err != nil {
			panic(err)
		}

		fmt.Printf("%s <- %s", addr.String(), string(buffer[:n]))

		n, err = l.WriteTo(buffer[:n], addr)

		if err != nil {
			panic(err)
		}

		fmt.Printf("%s -> %s", addr.String(), string(buffer[:n]))
	}
}
