package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"github.com/golang/protobuf/proto"
	"github.com/lucas-clemente/quic-go"
	"log"
)

type messageType byte

const (
	protocolVersion = byte(0x01)

	messageTypeRegisterRequest  = messageType(0x01)
	messageTypeRegisterResponse = messageType(0x02)

	messageTypeForwardRequest  = messageType(0x03)
	messageTypeForwardResponse = messageType(0x04)

	messageTypeKeepaliveRequest  = messageType(0x05)
	messageTypeKeepaliveResponse = messageType(0x06)
	messageTypeDataMessage       = messageType(0x07)

	messageSendBufferBytes    = 16 * 1024
	messageReceiveBufferBytes = 18 * 1024 // Account for protobuf overhead!
)

var messageTypes = map[messageType]string{
	messageTypeRegisterRequest:  "RegisterRequest",
	messageTypeRegisterResponse: "RegisterResponse",

	messageTypeForwardRequest:  "ForwardRequest",
	messageTypeForwardResponse: "ForwardResponse",

	messageTypeKeepaliveRequest:  "KeepaliveRequest",
	messageTypeKeepaliveResponse: "KeepaliveResponse",
	messageTypeDataMessage:       "DataMessage",
}

func sendmsg(stream quic.Stream, messageType messageType, message proto.Message) {
	messageBytes, err := proto.Marshal(message)
	if err != nil {
		panic(err)
	}

	messageTypeBytes := []byte{byte(messageType)}

	messageLengthBuf := make([]byte, 4)
	messageLengthLength := binary.PutVarint(messageLengthBuf, int64(len(messageBytes)))
	messageLengthBytes := messageLengthBuf[:messageLengthLength]

	stream.Write(messageTypeBytes)
	stream.Write(messageLengthBytes)
	stream.Write(messageBytes)

	log.Println("-> [" + messageTypes[messageType] + "] " + message.(proto.Message).String())
}

func recvmsg2(stream quic.Stream) (messageType, interface{}) {
	reader := bufio.NewReader(stream)

	messageTypeByte, err := reader.ReadByte()
	if err != nil {
		panic(err)
	}

	messageLength, err := binary.ReadVarint(reader)
	if err != nil {
		panic(err)
	}
	// TODO max len

	messageBytes := make([]byte, messageLength)
	read, err := reader.Read(messageBytes)
	if err != nil {
		panic(err)
	}

	if int64(read) != messageLength {
		panic(errors.New("invalid message len"))
	}

	messageType := messageType(messageTypeByte)

	switch messageType {
	case messageTypeRegisterRequest:
		return recvread(messageBytes, messageType, &RegisterRequest{})
	case messageTypeRegisterResponse:
		return recvread(messageBytes, messageType, &RegisterResponse{})
	case messageTypeForwardRequest:
		return recvread(messageBytes, messageType, &ForwardRequest{})
	case messageTypeForwardResponse:
		return recvread(messageBytes, messageType, &ForwardResponse{})
	case messageTypeKeepaliveRequest:
		return recvread(messageBytes, messageType, &KeepaliveRequest{})
	case messageTypeKeepaliveResponse:
		return recvread(messageBytes, messageType, &KeepaliveResponse{})
	case messageTypeDataMessage:
		return recvread(messageBytes, messageType, &DataMessage{})
	default:
		panic(errors.New("Unknown message"))
	}
}

func recvread(messageBytes []byte, msgType messageType, message proto.Message) (messageType, proto.Message) {
	err := proto.Unmarshal(messageBytes, message)
	if err != nil {
		panic(err)
	}

	log.Println("<- [" + messageTypes[msgType] + "] " + message.(proto.Message).String())
	return msgType, message
}
