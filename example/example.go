package main

import (
	"fmt"
	"heckel.io/natter"
	"io"
	"net"
	"time"
)

func main() {
	daemon := natter.NewDaemon()
	server := natter.NewServer()

	go server.Start(":10000")
	go daemon.Start("localhost:10000", "bob")

	go echoServer()

	time.Sleep(1 * time.Second)

	alice, _ := natter.NewClient("alice", "localhost:10000", nil)
	alice.Forward(2000, "bob", ":22")

	time.Sleep(1 * time.Second)

	go echoClient("Benny", 500)
	go echoClient("Lena", 600)

}

func echoClient(name string, wait int) {
	conn, _ := net.Dial("tcp", "localhost:20000")

	for i := 1; i <= 5; i++ {
		conn.Write([]byte(fmt.Sprintf("%s says %d", name, i)))
		time.Sleep(time.Duration(wait) * time.Millisecond)
	}

	conn.Close()
}
func echoServer() {
	listen, err := net.Listen("tcp", ":30000")
	if err != nil {
		panic(err)
	}

	for {
		conn, _ := listen.Accept()
		io.Copy(loggingWriter{conn}, conn)
	}
}

type loggingWriter struct {
	io.Writer
}

func (w loggingWriter) Write(b []byte) (int, error) {
	fmt.Printf("Server: Got '%s'\n", string(b))
	return w.Writer.Write(b)
}