package main

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"net"
)

type messageType byte

const (
	protocolVersion = byte(0x01)

	messageTypeRegisterRequest = messageType(0x01)
	messageTypeRegisterResponse = messageType(0x02)

	messageTypeGetRequest = messageType(0x03)
	messageTypeGetResponse = messageType(0x04)

	messageTypeChatMessage = messageType(0x05)
)

func sendmsg(conn *net.UDPConn, addr *net.UDPAddr, messageType messageType, message proto.Message) {
	bytes, err := proto.Marshal(message)
	if err != nil {
		panic(err)
	}

	fmt.Println("-> [" + string(messageType) + "] " + message.String())

	bytes = append([]byte{byte(messageType)}, bytes...)
	conn.WriteToUDP(bytes, addr)
}

func recvmsg() {

}
