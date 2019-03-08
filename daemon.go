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

type daemon struct {
	hubAddr      *net.UDPAddr
	hubStream    quic.Stream
	localUdpConn net.PacketConn

	// TODO this should be an array
	connectionId string
	peerUdpAddr *net.UDPAddr
	forwardConn net.Conn
}

func (d *daemon) start(hubAddr string, source string) {
	var err error

	// Resolve hub address
	d.hubAddr, err = net.ResolveUDPAddr("udp4", hubAddr)
	if err != nil {
		panic(err)
	}

	// Listen to local UDP address
	rand.Seed(time.Now().Unix())
	localPort := fmt.Sprintf(":%d", 10000+rand.Intn(10000))
	d.localUdpConn, err = net.ListenPacket("udp", localPort)
	if err != nil {
		panic(err)
	}

	// Open connection to hub
	session, err := quic.Dial(d.localUdpConn, d.hubAddr, hubAddr, &tls.Config{InsecureSkipVerify: true},
		&quic.Config{
			KeepAlive: true,
			ConnectionIDLength: 8,
			Versions: []quic.VersionNumber{quic.VersionGQUIC43},
	})

	if err != nil {
		panic(err)
	}

	d.hubStream, err = session.OpenStream()
	if err != nil {
		panic(err)
	}

	go d.listenHubStream()
	go func() {
		for {
			sendmsg(d.hubStream, messageTypeRegisterRequest, &RegisterRequest{Source: source})
			time.Sleep(15 * time.Second)
		}
	}()

	listener, err := quic.Listen(d.localUdpConn, generateTLSConfig(), &quic.Config{KeepAlive:true})
	if err != nil {
		panic(err)
	}

	log.Println("Waiting for connections")
	for {
		session, err := listener.Accept()
		if err != nil {
			panic(err)
		}

		go d.handlePeerSession(session)
	}
}

func (d *daemon) handlePeerSession(session quic.Session) {
	for {
		stream, err := session.AcceptStream()
		if err != nil {
			panic(err)
		}

		go d.handlePeerStream(session, stream)
	}
}

func (d *daemon) handlePeerStream(session quic.Session, stream quic.Stream) {
	go func() { io.Copy(stream, d.forwardConn) }()
	go func() { io.Copy(d.forwardConn, stream) }()
}

func (d *daemon) listenHubStream() {
	var err error

	for {
		messageType, message := recvmsg2(d.hubStream)

		switch messageType {
		case messageTypeRegisterResponse:
			// Nothing
		case messageTypeForwardRequest:
			request, _ := message.(*ForwardRequest)
			log.Println(">", request.Target)

			d.forwardConn, err = net.Dial("tcp", request.TargetForwardAddr)
			if err != nil {
				panic(err)
			}

			d.peerUdpAddr, err = net.ResolveUDPAddr("udp4", request.SourceAddr)
			if err != nil {
				panic(err)
			}

			d.connectionId = request.Id

			sendmsg(d.hubStream, messageTypeForwardResponse, &ForwardResponse{
				Id: request.Id,
				Success: true,
				Source: request.Source,
				SourceAddr: request.SourceAddr,
				Target: request.Target,
				TargetAddr: request.TargetAddr,
			})

			go func() {
				for {
					d.localUdpConn.WriteTo([]byte("ping"), d.peerUdpAddr)
					time.Sleep(5 * time.Second)
				}
			}()
		}
	}
}
