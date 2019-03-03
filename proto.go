package main

import (
	"errors"
	"github.com/golang/protobuf/proto"
	"log"
	"net"
)

type messageType byte

const (
	protocolVersion = byte(0x01)

	messageTypeRegisterRequest = messageType(0x01)
	messageTypeRegisterResponse = messageType(0x02)

	messageTypeForwardRequest  = messageType(0x03)
	messageTypeForwardResponse = messageType(0x04)

	messageTypeKeepaliveRequest = messageType(0x05)
	messageTypeKeepaliveResponse = messageType(0x06)
	messageTypeDataMessage = messageType(0x07)

	messageSendBufferBytes = 16 * 1024
	messageReceiveBufferBytes = 18 * 1024 // Account for protobuf overhead!
)

var messageTypes = map[messageType]string {
	messageTypeRegisterRequest: "RegisterRequest",
	messageTypeRegisterResponse: "RegisterResponse",

	messageTypeForwardRequest:  "ForwardRequest",
	messageTypeForwardResponse: "ForwardResponse",

	messageTypeKeepaliveRequest: "KeepaliveRequest",
	messageTypeKeepaliveResponse: "KeepaliveResponse",
	messageTypeDataMessage: "DataMessage",
}

func sendmsg(conn *net.UDPConn, addr *net.UDPAddr, messageType messageType, message proto.Message) {
	bytes, err := proto.Marshal(message)
	if err != nil {
		panic(err)
	}

	log.Println("-> [" + messageTypes[messageType] + "] " + message.String())

	bytes = append([]byte{byte(messageType)}, bytes...)
	conn.WriteToUDP(bytes, addr)
}

func recvmsg(conn *net.UDPConn) (from *net.UDPAddr, msgType messageType, message proto.Message) {
	buf := make([]byte, messageReceiveBufferBytes)
	n, addr, err := conn.ReadFromUDP(buf)
	if n == 0 || err != nil {
		panic(err)
	}

	msgType = messageType(buf[0])

	switch msgType {
	case messageTypeRegisterRequest:
		var message RegisterRequest
		err = proto.Unmarshal(buf[1:n], &message)
		if err != nil {
			panic(err)
		}

		log.Println("<- [" + messageTypes[msgType] + "] " + message.String())
		return addr, msgType, &message
	case messageTypeRegisterResponse:
		var message RegisterResponse
		err = proto.Unmarshal(buf[1:n], &message)
		if err != nil {
			panic(err)
		}

		log.Println("<- [" + messageTypes[msgType] + "] " + message.String())
		return addr, msgType, &message
	case messageTypeForwardRequest:
		var message ForwardRequest
		err = proto.Unmarshal(buf[1:n], &message)
		if err != nil {
			panic(err)
		}

		log.Println("<- [" + messageTypes[msgType] + "] " + message.String())
		return addr, msgType, &message
	case messageTypeForwardResponse:
		var message ForwardResponse
		err = proto.Unmarshal(buf[1:n], &message)
		if err != nil {
			panic(err)
		}

		log.Println("<- [" + messageTypes[msgType] + "] " + message.String())
		return addr, msgType, &message
	case messageTypeKeepaliveRequest:
		var message KeepaliveRequest
		err = proto.Unmarshal(buf[1:n], &message)
		if err != nil {
			panic(err)
		}

		log.Println("<- [" + messageTypes[msgType] + "] " + message.String())
		return addr, msgType, &message
	case messageTypeKeepaliveResponse:
		var message KeepaliveResponse
		err = proto.Unmarshal(buf[1:n], &message)
		if err != nil {
			panic(err)
		}

		log.Println("<- [" + messageTypes[msgType] + "] " + message.String())
		return addr, msgType, &message
	case messageTypeDataMessage:
		var message DataMessage
		err = proto.Unmarshal(buf[1:n], &message)
		if err != nil {
			panic(err)
		}

		log.Println("<- [" + messageTypes[msgType] + "] " + message.String())
		return addr, msgType, &message
	default:
		panic(errors.New("Unknown message"))
	}
}
