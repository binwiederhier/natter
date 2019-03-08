package main

import (
	"crypto/tls"
	"fmt"
	"github.com/lucas-clemente/quic-go"
	"io"
	"log"
	"math/rand"
	"net"
	"time"
)

type forward struct {
	hubAddr          *net.UDPAddr
	hubStream        quic.Stream

	// TODO FIXME This should be per connection!
	connectionId     string
	localTcpListener net.Listener
	localUdpConn     net.PacketConn
	peerUdpAddr      *net.UDPAddr
	peerStream       quic.Stream
}

func (f *forward) start(hubAddr string, source string, sourcePort int, target string, targetForwardAddr string) {
	var err error

	// Resolve hub address
	f.hubAddr, err = net.ResolveUDPAddr("udp4", hubAddr)
	if err != nil {
		panic(err)
	}

	// TODO This only supports one forward and one connection!!!

	// Listen to local UDP address
	rand.Seed(time.Now().Unix())
	localUdpPort := fmt.Sprintf(":%d", 10000+rand.Intn(10000))
	f.localUdpConn, err = net.ListenPacket("udp", localUdpPort)
	if err != nil {
		panic(err)
	}

	session, err := quic.Dial(f.localUdpConn, f.hubAddr, hubAddr, &tls.Config{InsecureSkipVerify: true},
		&quic.Config{
			KeepAlive: true,
			ConnectionIDLength: 8,
			Versions: []quic.VersionNumber{quic.VersionGQUIC43,
		}})

	if err != nil {
		panic(err)
	}

	f.hubStream, err = session.OpenStream()
	if err != nil {
		panic(err)
	}

	// Listen to local TCP address
	f.localTcpListener, err = net.Listen("tcp", fmt.Sprintf(":%d", sourcePort))
	if err != nil {
		panic(err)
	}

	go f.readHub()
	go f.writeHub(source, sourcePort, target, targetForwardAddr)
	go f.listenTcp()

	for {
		time.Sleep(30 * time.Second)
	}
}

func (f *forward) writeHub(source string, sourcePort int, target string, targetForwardAddr string) {
	log.Printf("Requesting connection to %s:%d\n", target, targetForwardAddr)

	sendmsg(f.hubStream, messageTypeForwardRequest, &ForwardRequest{
		Source: source,
		Target: target,
		TargetForwardAddr: targetForwardAddr,
	})
}

func (f *forward) readHub() {
	for {
		messageType, message := recvmsg2(f.hubStream)

		switch messageType {
		case messageTypeForwardResponse:
			response, _ := message.(*ForwardResponse)

			if response.Success {
				var err error
				log.Print("Peer address: ", response.TargetAddr)

				f.peerUdpAddr, err = net.ResolveUDPAddr("udp4", response.TargetAddr)
				if err != nil {
					panic(err)
				}


				f.connectionId = response.Id

				go f.openPeerStream()
			} else {
				log.Println("Failed forward response")
			}

		}
	}
}

func (f *forward) listenTcp() {
	for f.peerStream == nil { // TODO racy
		log.Println("Cannot forward yet. UDP connection not active yet.")
		time.Sleep(1 * time.Second)
	}

	for {
		conn, err := f.localTcpListener.Accept()
		if err != nil {
			panic(err)
		}

		go func() { io.Copy(f.peerStream, conn) }()
		go func() { io.Copy(conn, f.peerStream) }()
	}
}

func (f *forward) openPeerStream() {
	for {
		peerHost := fmt.Sprintf("%s:%d", f.peerUdpAddr.IP.String(), f.peerUdpAddr.Port)
		session, err := quic.Dial(f.localUdpConn, f.peerUdpAddr,peerHost, &tls.Config{InsecureSkipVerify: true},
			&quic.Config{
				KeepAlive:true,
				ConnectionIDLength: 8,
				Versions: []quic.VersionNumber{quic.VersionGQUIC43},
		})

		if err != nil {
			panic(err)
		}

		f.peerStream, err = session.OpenStreamSync()

		if err != nil {
			log.Println("Not connected yet.")
			time.Sleep(1)
			continue
		}

		break
	}

	log.Println("Connected!")
}