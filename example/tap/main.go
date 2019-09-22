package main

import (
	"encoding/hex"
	"github.com/songgao/packets/ethernet"
	"github.com/songgao/water"
	"github.com/vishvananda/netlink"
	"io"
	"log"
)

func main() {
	tap0 := water.Config{
		DeviceType: water.TAP,
	}
	tap0.Name = "tap10"

	ifce0, err := water.New(tap0)
	if err != nil {
		log.Fatal(err)
	}

	tap1 := water.Config{
		DeviceType: water.TAP,
	}
	tap1.Name = "tap11"

	ifce1, err := water.New(tap1)
	if err != nil {
		log.Fatal(err)
	}


	tapLink0, _ := netlink.LinkByName("tap10")
	netlink.LinkSetUp(tapLink0)

	tapLink1, _ := netlink.LinkByName("tap11")
	netlink.LinkSetUp(tapLink1)


	go func() { io.Copy(loggingWriter{ifce0}, ifce1) }()
	go func() { io.Copy(loggingWriter{ifce1}, ifce0) }()

	var frame ethernet.Frame

	for {
		frame.Resize(1500)
		n, err := ifce0.Read([]byte(frame))
		if err != nil {
			log.Fatal(err)
		}
		frame = frame[:n]
		log.Printf("Dst: %s\n", frame.Destination())
		log.Printf("Src: %s\n", frame.Source())
		log.Printf("Ethertype: % x\n", frame.Ethertype())
		log.Printf("Payload: % x\n", frame.Payload())
		frame.Ethertype()
	}
	// io.Copy(loggingWriter{ioutil.Discard}, ifce0)

	select {}
	/*var frame ethernet.Frame

	for {
		frame.Resize(1500)
		n, err := ifce.Read([]byte(frame))
		if err != nil {
			log.Fatal(err)
		}
		frame = frame[:n]
		log.Printf("Dst: %s\n", frame.Destination())
		log.Printf("Src: %s\n", frame.Source())
		log.Printf("Ethertype: % x\n", frame.Ethertype())
		log.Printf("Payload: % x\n", frame.Payload())
		frame.Ethertype()
	}*/
}


type loggingWriter struct {
	io.Writer
}

func (w loggingWriter) Write(b []byte) (int, error) {
	n, err := w.Writer.Write(b)
	println(hex.Dump(b))
	return n, err
}
