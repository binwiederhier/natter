package natter

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"github.com/golang/protobuf/proto"
	"github.com/lucas-clemente/quic-go"
	"io"
	"log"
	"math/big"
	"sync"
)

type messageType byte

const (
	messageTypeRegisterRequest  = messageType(0x01)
	messageTypeRegisterResponse = messageType(0x02)

	messageTypeForwardRequest  = messageType(0x03)
	messageTypeForwardResponse = messageType(0x04)
)

var messageTypes = map[messageType]string{
	messageTypeRegisterRequest:  "RegisterRequest",
	messageTypeRegisterResponse: "RegisterResponse",

	messageTypeForwardRequest:  "ForwardRequest",
	messageTypeForwardResponse: "ForwardResponse",
}

type messenger struct {
	stream quic.Stream
	sendmu sync.Mutex
	receivemu sync.Mutex
}

func (messenger *messenger) send(messageType messageType, message proto.Message) error {
	messenger.sendmu.Lock()
	defer messenger.sendmu.Unlock()

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

	_, err = messenger.stream.Write(send)
	if err != nil {
		return err
	}

	log.Println("-> [" + messageTypes[messageType] + "] " + message.(proto.Message).String())
	return nil
}

func (messenger *messenger) receive() (messageType, proto.Message, error) {
	messenger.receivemu.Lock()
	defer messenger.receivemu.Unlock()

	var (
		messageTypeBytes = make([]byte, 1)
		messageLengthBytes = make([]byte, 4)
		messageLength uint32
		messageBytes []byte
	)

	if _, err := io.ReadFull(messenger.stream, messageTypeBytes); err != nil {
		return 0, nil, err
	}

	if _, err := io.ReadFull(messenger.stream, messageLengthBytes); err != nil {
		return 0, nil, err
	}

	if messageLength = binary.BigEndian.Uint32(messageLengthBytes); messageLength > 8192 {
		return 0, nil, errors.New("message too long")
	}

	messageBytes = make([]byte, messageLength)
	if _, err := io.ReadFull(messenger.stream, messageBytes); err != nil {
		return 0, nil, err
	}

	messageType := messageType(messageTypeBytes[0])

	var message proto.Message

	switch messageType {
	case messageTypeRegisterRequest:
		message = &RegisterRequest{}
	case messageTypeRegisterResponse:
		message = &RegisterResponse{}
	case messageTypeForwardRequest:
		message = &ForwardRequest{}
	case messageTypeForwardResponse:
		message = &ForwardResponse{}
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

func (messenger *messenger) close() error {
	return messenger.stream.Close()
}

// Setup a bare-bones TLS config for the server
func generateTlsConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{Certificates: []tls.Certificate{tlsCert}}
}

func generateQuicTlsClientConfig() *tls.Config {
	return &tls.Config{InsecureSkipVerify: true}
}

func generateQuicConfig() *quic.Config {
	return &quic.Config{
		KeepAlive:          true,
		ConnectionIDLength: 8,
		Versions: []quic.VersionNumber{quic.VersionGQUIC43},
	}
}