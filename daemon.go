package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"time"
)

type daemon struct {
	hubAddr      *net.UDPAddr
	localUdpConn *net.UDPConn
	localUdpAddr *net.UDPAddr

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
	d.localUdpAddr, err = net.ResolveUDPAddr("udp4", localPort)
	if err != nil {
		panic(err)
	}

	d.localUdpConn, err = net.ListenUDP("udp", d.localUdpAddr)
	if err != nil {
		panic(err)
	}

	go d.listen(d.localUdpConn)

	for {
		sendmsg(d.localUdpConn, d.hubAddr, messageTypeRegisterRequest,
			&RegisterRequest{Source: source})

		time.Sleep(15 * time.Second)
	}
}

func (d *daemon) listen(conn *net.UDPConn) {
	var err error

	for {
		addr, messageType, message := recvmsg(conn)

		switch messageType {
		case messageTypeRegisterResponse:
			// Nothing
		case messageTypeKeepaliveRequest:
			request, _ := message.(*KeepaliveRequest)
			sendmsg(d.localUdpConn, addr, messageTypeKeepaliveResponse, &KeepaliveResponse{
				Id: request.Id,
				Rand: request.Rand,
			})
		case messageTypeDataMessage:
			msg, _ := message.(*DataMessage)
			d.forwardConn.Write(msg.Data) // TODO Identify correct conn, do this in go routine
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

			sendmsg(conn, addr, messageTypeForwardResponse, &ForwardResponse{
				Id: request.Id,
				Success: true,
				Source: request.Source,
				SourceAddr: request.SourceAddr,
				Target: request.Target,
				TargetAddr: request.TargetAddr,
			})

			go d.keepalive()
		}
	}
}

func (d *daemon) keepalive() {
	for {
		sendmsg(d.localUdpConn, d.peerUdpAddr, messageTypeKeepaliveRequest, &KeepaliveRequest{
			Id: d.connectionId,
			Rand: rand.Int31(),
		})

		time.Sleep(10 * time.Second)
	}
}