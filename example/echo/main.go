package main

import (
	"fmt"
	"heckel.io/natter"
	"io"
	"net"
	"time"
)

func main() {
	// Start broker
	broker, _ := natter.NewBroker(&natter.Config{BrokerAddr: ":5000"})
	go broker.ListenAndServe()

	// Start listening client "Bob", offering two echo servers at :7000 and :7001
	go echoServer(":7000")
	go echoServer(":7001")

	bob, _ := natter.NewClient(&natter.Config{ClientId: "bob", BrokerAddr: "localhost:5000"})
	bob.Listen()

	// Start forwarding client "Alice" and her two echo clients
	alice, _ := natter.NewClient(&natter.Config{ClientId: "alice", BrokerAddr: "localhost:5000"})
	alice.Forward(":6000", "bob", ":7000", nil)
	alice.Forward(":6001", "bob", ":7001", nil)

	go echoClient("localhost:6000", "Alice Zero", 500)
	go echoClient("localhost:6001", "Alice One", 600)

	time.Sleep(5 * time.Second)
}

func echoClient(server string, name string, wait int) {
	conn, _ := net.Dial("tcp", server)

	for i := 1; i <= 5; i++ {
		_, err := conn.Write([]byte(fmt.Sprintf("%s says %d", name, i)))
		if err  != nil {
			panic(err)
		}

		time.Sleep(time.Duration(wait) * time.Millisecond)
	}

	conn.Close()
}

func echoServer(listenAddr string) {
	listen, err := net.Listen("tcp", listenAddr)
	if err != nil {
		panic(err)
	}

	for {
		conn, _ := listen.Accept()
		go io.Copy(loggingWriter{listenAddr, conn.RemoteAddr().String(), conn}, conn)
	}
}

type loggingWriter struct {
	listenAddr string
	remoteAddr string
	io.Writer
}

func (w loggingWriter) Write(b []byte) (int, error) {
	fmt.Printf("Server [%s]: Got '%s' from '%s'\n", w.listenAddr, string(b), w.remoteAddr)
	return w.Writer.Write(b)
}