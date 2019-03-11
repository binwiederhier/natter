package natter

import (
	"fmt"
	"github.com/lucas-clemente/quic-go"
	"io"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"
)

type daemon struct {
	hubAddr   *net.UDPAddr
	hubStream quic.Stream
	udpConn   net.PacketConn

	fwmu sync.Mutex
	forwards map[string]*forward
}

type forward struct {
	connectionId string
	peerUdpAddr  *net.UDPAddr
	targetForwardAddr string
}

func NewDaemon() *daemon {
	return &daemon{}
}

func (d *daemon) Start(hubAddr string, source string) {
	d.forwards = make(map[string]*forward)

	var err error

	// Resolve hub address
	d.hubAddr, err = net.ResolveUDPAddr("udp4", hubAddr)
	if err != nil {
		panic(err)
	}

	// Listen to local UDP address
	localPort := fmt.Sprintf(":%d", 10000+rand.Intn(10000))
	d.udpConn, err = net.ListenPacket("udp", localPort)
	if err != nil {
		panic(err)
	}

	// Open connection to hub
	session, err := quic.Dial(d.udpConn, d.hubAddr, hubAddr, generateQuicTlsClientConfig(), generateQuicConfig())

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

	listener, err := quic.Listen(d.udpConn, generateTLSConfig(), &quic.Config{KeepAlive: true})
	if err != nil {
		panic(err)
	}

	for {
		log.Println("[daemon] Waiting for connections")
		session, err := listener.Accept()
		if err != nil {
			panic(err)
		}

		go d.handlePeerSession(session)
	}
}

func (d *daemon) handlePeerSession(session quic.Session) {
	log.Println("[daemon] Session from " + session.RemoteAddr().String() + " accepted.")

	peerAddr := session.RemoteAddr().(*net.UDPAddr)

	d.fwmu.Lock()
	forward, ok := d.forwards[peerAddr.String()]
	d.fwmu.Unlock()

	log.Println("[daemon] Client accepted from " + peerAddr.String() + ", forward found to " + forward.targetForwardAddr)

	if !ok {
		log.Printf("[daemon] Session from unexpected client %s. Closing.", peerAddr.String())
		session.Close()
		return
	}

	for {
		stream, err := session.AcceptStream()
		if err != nil {
			panic(err)
		}

		go d.handlePeerStream(session, stream, *forward)
	}
}

func (d *daemon) handlePeerStream(session quic.Session, stream quic.Stream, forward forward) {
	log.Printf("[daemon] Stream %d accepted. Starting to forward.\n", stream.StreamID())

	forwardConn, err := net.Dial("tcp", forward.targetForwardAddr)
	if err != nil {
		panic(err)
	}

	go func() { io.Copy(stream, forwardConn) }()
	go func() { io.Copy(forwardConn, stream) }()
}

func (d *daemon) listenHubStream() {
	for {
		messageType, message := recvmsg2(d.hubStream)

		switch messageType {
		case messageTypeRegisterResponse:
			// Nothing
		case messageTypeForwardRequest:
			request, _ := message.(*ForwardRequest)
			log.Printf("[daemon] Accepted forward request from %s to TCP addr %s", request.Source, request.TargetForwardAddr)

			peerUdpAddr, err := net.ResolveUDPAddr("udp4", request.SourceAddr)
			if err != nil {
				panic(err)
			}

			forward := &forward{
				connectionId: request.Id,
				targetForwardAddr: request.TargetForwardAddr,
				peerUdpAddr: peerUdpAddr,
			}

			d.fwmu.Lock()
			d.forwards[peerUdpAddr.String()] = forward
			d.fwmu.Unlock()

			sendmsg(d.hubStream, messageTypeForwardResponse, &ForwardResponse{
				Id:         request.Id,
				Success:    true,
				Source:     request.Source,
				SourceAddr: request.SourceAddr,
				Target:     request.Target,
				TargetAddr: request.TargetAddr,
			})

			go func() {
				for {
					d.udpConn.WriteTo([]byte("ping"), peerUdpAddr)
					time.Sleep(5 * time.Second)
				}
			}()
		}
	}
}
