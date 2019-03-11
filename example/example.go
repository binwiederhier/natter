package main

import (
	"fmt"
	"heckel.io/natter"
	"io"
	"net"
	"time"
)

func main() {
	server := natter.NewServer()
	go server.Start(":5000")
	go echoServer()

	time.Sleep(1 * time.Second)

	bob, _ := natter.NewClient(&natter.ClientConfig{ClientUser: "bob", BrokerAddr: "localhost:5000"})
	bob.Listen()

	time.Sleep(1 * time.Second)

	alice, _ := natter.NewClient(&natter.ClientConfig{ClientUser: "alice", BrokerAddr: "localhost:5000"})
	alice.Forward(":6000", "bob", ":7000")
	alice.Forward(":6001", "bob", ":7000")

	time.Sleep(1 * time.Second)

	go echoClient("localhost:6000", "Benny", 500)
	go echoClient("localhost:6001", "Lena", 600)

	time.Sleep(5 * time.Second)
}

func echoClient(server string, name string, wait int) {
	conn, _ := net.Dial("tcp", server)

	for i := 1; i <= 5; i++ {
		conn.Write([]byte(fmt.Sprintf("%s says %d", name, i)))
		time.Sleep(time.Duration(wait) * time.Millisecond)
	}

	conn.Close()
}

func echoServer() {
	listen, err := net.Listen("tcp", ":7000")
	if err != nil {
		panic(err)
	}

	for {
		conn, _ := listen.Accept()
		go io.Copy(loggingWriter{conn}, conn)
	}
}

type loggingWriter struct {
	io.Writer
}

func (w loggingWriter) Write(b []byte) (int, error) {
	fmt.Printf("Server: Got '%s'\n", string(b))
	return w.Writer.Write(b)
}