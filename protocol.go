package natter

import (
	"encoding/binary"
	"errors"
	"github.com/golang/protobuf/proto"
	"github.com/lucas-clemente/quic-go"
	"heckel.io/natter/internal"
	"io"
	"log"
	"sync"
)

type messageType byte

const (
	messageTypeCheckinRequest  = messageType(0x01)
	messageTypeCheckinResponse = messageType(0x02)

	messageTypeForwardRequest  = messageType(0x03)
	messageTypeForwardResponse = messageType(0x04)
)

var messageTypes = map[messageType]string{
	messageTypeCheckinRequest:  "CheckinRequest",
	messageTypeCheckinResponse: "CheckinResponse",

	messageTypeForwardRequest:  "ForwardRequest",
	messageTypeForwardResponse: "ForwardResponse",
}

type protocol struct {
	stream quic.Stream
	sendmu sync.Mutex
	receivemu sync.Mutex
}

func (p *protocol) send(messageType messageType, message proto.Message) error {
	p.sendmu.Lock()
	defer p.sendmu.Unlock()

	messageBytes, err := proto.Marshal(message)
	if err != nil {
		return err
	}

	messageTypeBytes := []byte{byte(messageType)}

	messageLengthBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(messageLengthBytes, uint32(len(messageBytes)))

	send := messageTypeBytes
	send = append(send, messageLengthBytes[:]...)
	send = append(send, messageBytes[:]...)

	_, err = p.stream.Write(send)
	if err != nil {
		return err
	}

	log.Println("-> [" + messageTypes[messageType] + "] " + message.(proto.Message).String())
	return nil
}

func (p *protocol) receive() (messageType, proto.Message, error) {
	p.receivemu.Lock()
	defer p.receivemu.Unlock()

	var (
		messageTypeBytes = make([]byte, 1)
		messageLengthBytes = make([]byte, 4)
		messageLength uint32
		messageBytes []byte
	)

	if _, err := io.ReadFull(p.stream, messageTypeBytes); err != nil {
		return 0, nil, err
	}

	if _, err := io.ReadFull(p.stream, messageLengthBytes); err != nil {
		return 0, nil, err
	}

	if messageLength = binary.BigEndian.Uint32(messageLengthBytes); messageLength > 8192 {
		return 0, nil, errors.New("message too long")
	}

	messageBytes = make([]byte, messageLength)
	if _, err := io.ReadFull(p.stream, messageBytes); err != nil {
		return 0, nil, err
	}

	messageType := messageType(messageTypeBytes[0])

	var message proto.Message

	switch messageType {
	case messageTypeCheckinRequest:
		message = &internal.CheckinRequest{}
	case messageTypeCheckinResponse:
		message = &internal.CheckinResponse{}
	case messageTypeForwardRequest:
		message = &internal.ForwardRequest{}
	case messageTypeForwardResponse:
		message = &internal.ForwardResponse{}
	default:
		return 0, nil, errors.New("Unknown message")
	}

	err := proto.Unmarshal(messageBytes, message)
	if err != nil {
		return 0, nil, err
	}

	log.Println("<- [" + messageTypes[messageType] + "] " + message.(proto.Message).String())
	return messageType, message, nil
}

func (p *protocol) close() error {
	return p.stream.Close()
}
