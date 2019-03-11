package natter

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"github.com/golang/protobuf/proto"
	"github.com/lucas-clemente/quic-go"
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

func (messenger *messenger) send(messageType messageType, message proto.Message) {
	messenger.sendmu.Lock()
	defer messenger.sendmu.Unlock()

	messageBytes, err := proto.Marshal(message)
	if err != nil {
		panic(err)
	}

	messageTypeBytes := []byte{byte(messageType)}

	messageLengthBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(messageLengthBytes, uint32(len(messageBytes)))

	messenger.stream.Write(messageTypeBytes)
	messenger.stream.Write(messageLengthBytes)
	messenger.stream.Write(messageBytes)

	log.Println("-> [" + messageTypes[messageType] + "] " + message.(proto.Message).String())
}

func (messenger *messenger) receive() (messageType, proto.Message, error) {
	messenger.receivemu.Lock()
	defer messenger.receivemu.Unlock()

	reader := bufio.NewReader(messenger.stream)

	messageTypeByte, err := reader.ReadByte()

	if err != nil {
		return 0, nil, err
	}

	messageLengthBytes := make([]byte, 4)
	n, err := reader.Read(messageLengthBytes)
	if err != nil {
		return 0, nil, err
	}

	if n != len(messageLengthBytes) {
		return 0, nil, errors.New("cannot read message length")
	}

	messageLength := binary.BigEndian.Uint32(messageLengthBytes)

	if messageLength > 1 * 1024 * 1024 {
		return 0, nil, errors.New("message too large: " + string(messageLength))
	}

	messageBytes := make([]byte, messageLength)
	read, err := reader.Read(messageBytes)
	if err != nil {
		panic(err)
	}

	if read != int(messageLength) {
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
	default:
		panic(errors.New("Unknown message"))
	}
}

func (messenger *messenger) close() {
	messenger.stream.Close()
}

func recvread(messageBytes []byte, msgType messageType, message proto.Message) (messageType, proto.Message, error) {
	err := proto.Unmarshal(messageBytes, message)
	if err != nil {
		return 0, nil, err
	}

	log.Println("<- [" + messageTypes[msgType] + "] " + message.(proto.Message).String())
	return msgType, message, nil
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