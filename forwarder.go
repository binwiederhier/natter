package natter

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

type forwarder struct {
	udpConn          net.PacketConn

	hubAddr   *net.UDPAddr
	hubStream quic.Stream

	connectionId     string
	peerUdpAddr      *net.UDPAddr
	//peerStream       quic.Stream
	localTcpListener net.Listener
}

type fconn struct {
}

func (f *forwarder) Start(hubAddr string, source string, sourcePort int, target string, targetForwardAddr string) {
	var err error

	// Resolve hub address
	f.hubAddr, err = net.ResolveUDPAddr("udp4", hubAddr)
	if err != nil {
		panic(err)
	}

	// TODO This only supports one forward and one connection!!!

	// Listen to local UDP address
	udpAddr := fmt.Sprintf(":%d", 10000+rand.Intn(10000))
	log.Printf("[forwarder] Listening on UDP address %s\n", udpAddr)

	f.udpConn, err = net.ListenPacket("udp", udpAddr)
	if err != nil {
		panic(err)
	}

	log.Printf("[forwarder] Connecting to hub at %s\n", hubAddr)
	session, err := quic.Dial(f.udpConn, f.hubAddr, hubAddr, &tls.Config{InsecureSkipVerify: true},
		&quic.Config{
			KeepAlive:          true,
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
	localTcpAddr := fmt.Sprintf(":%d", sourcePort)
	log.Printf("[forwarder] Listening on local TCP address %s\n", localTcpAddr)
	f.localTcpListener, err = net.Listen("tcp", localTcpAddr)
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

func (f *forwarder) writeHub(source string, sourcePort int, target string, targetForwardAddr string) {
	log.Printf("[forwarder] Requesting connection via hub to target %s on TCP address %s\n", target, targetForwardAddr)

	sendmsg(f.hubStream, messageTypeForwardRequest, &ForwardRequest{
		Source:            source,
		Target:            target,
		TargetForwardAddr: targetForwardAddr,
	})
}

func (f *forwarder) readHub() {
	for {
		messageType, message := recvmsg2(f.hubStream)

		switch messageType {
		case messageTypeForwardResponse:
			response, _ := message.(*ForwardResponse)

			if response.Success {
				var err error
				log.Print("[forwarder] Peer address: ", response.TargetAddr)

				f.peerUdpAddr, err = net.ResolveUDPAddr("udp4", response.TargetAddr)
				if err != nil {
					panic(err)
				}

				f.connectionId = response.Id

				go func() {
					for {
						f.udpConn.WriteTo([]byte("ping"), f.peerUdpAddr)
						time.Sleep(5 * time.Second)
					}
				}()
			} else {
				log.Println("Failed forward response")
			}

		}
	}
}

func (f *forwarder) listenTcp() {
	for {
		conn, err := f.localTcpListener.Accept()
		if err != nil {
			panic(err)
		}

		log.Println("[forwarder] Client connected on TCP socket, opening stream ...")
		go f.openPeerStream(conn)
	}
}

func (f *forwarder) openPeerStream(conn net.Conn) {
	log.Print("[forwarder] Opening stream to peer")

	var peerStream quic.Stream

	for {
		peerHost := fmt.Sprintf("%s:%d", f.peerUdpAddr.IP.String(), f.peerUdpAddr.Port)
		session, err := quic.Dial(f.udpConn, f.peerUdpAddr, peerHost, &tls.Config{InsecureSkipVerify: true},
			&quic.Config{
				KeepAlive:          true,
				ConnectionIDLength: 8,
				Versions:           []quic.VersionNumber{quic.VersionGQUIC43},
			})

		if err != nil {
			panic(err)
		}

		peerStream, err = session.OpenStreamSync()

		if err != nil {
			log.Println("[forwarder] Not connected yet.")
			time.Sleep(1)
			continue
		}

		break
	}

	log.Println("[forwarder] Connected. Starting to forward.")

	go func() { io.Copy(peerStream, conn) }()
	go func() { io.Copy(conn, peerStream) }()

}
