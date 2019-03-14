package main

import (
	"fmt"
	"heckel.io/natter"
	"io"
	"net"
	"time"
)

func main() {
	startBroker()
	startBob()
	startAlice()

	time.Sleep(5 * time.Second)
}

func startBroker() {
	broker := natter.NewBroker()
	go broker.ListenAndServe(":5000")
}

func startAlice() {
	alice, _ := natter.NewClient(&natter.ClientConfig{ClientUser: "alice", BrokerAddr: "localhost:5000"})
	alice.Forward(":6000", "bob", ":7000")
	alice.Forward(":6001", "bob", ":7000")

	go startAlicesEchoClient("localhost:6000", "Young Alice ", 500)
	go startAlicesEchoClient("localhost:6001", "Old Alice", 600)
}

func startBob() {
	go startBobsEchoServer()

	bob, _ := natter.NewClient(&natter.ClientConfig{ClientUser: "bob", BrokerAddr: "localhost:5000"})
	bob.ListenIncoming()
}

func startAlicesEchoClient(server string, name string, wait int) {
	conn, _ := net.Dial("tcp", server)

	for i := 1; i <= 5; i++ {
		conn.Write([]byte(fmt.Sprintf("%s says %d", name, i)))
		time.Sleep(time.Duration(wait) * time.Millisecond)
	}

	conn.Close()
}

func startBobsEchoServer() {
	listen, err := net.Listen("tcp", ":7000")
	if err != nil {
		panic(err)
	}

	for {
		conn, _ := listen.Accept()
		go io.Copy(loggingWriter{conn.RemoteAddr().String(), conn}, conn)
	}
}

type loggingWriter struct {
	remoteAddr string
	io.Writer
}

func (w loggingWriter) Write(b []byte) (int, error) {
	fmt.Printf("Server: Got '%s' from '%s'\n", string(b), w.remoteAddr)
	return w.Writer.Write(b)
}