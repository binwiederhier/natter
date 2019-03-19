package natter

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"github.com/lucas-clemente/quic-go"
	"io/ioutil"
	"math/big"
	"os"
	"regexp"
)

func LoadConfig(filename string) (*Config, error) {
	raw, err := loadRawConfig(filename)
	if err != nil {
		return nil, err
	}

	config := &Config{}

	clientUser, ok := raw["ClientUser"]
	if ok {
		config.ClientUser = clientUser
	}

	brokerAddr, ok := raw["BrokerAddr"]
	if ok {
		config.BrokerAddr = brokerAddr
	}

	certificateFile, certificateOk := raw["Certificate"]
	privateKeyFile, privateKeyOk := raw["PrivateKey"]

	if certificateOk && privateKeyOk {
		certificatePem, err := ioutil.ReadFile(certificateFile)
		if err != nil {
			return nil, errors.New("invalid config file, Certificate setting is invalid, cannot read file")
		}

		privateKeyPem, err := ioutil.ReadFile(privateKeyFile)
		if err != nil {
			return nil, errors.New("invalid config file, PrivateKey setting is invalid, cannot read file")
		}

		keyPair, err := tls.X509KeyPair(certificatePem, privateKeyPem)
		if err != nil {
			return nil, errors.New("invalid config file, cannot decode certificate and/or private key")
		}

		config.TLSServerConfig = &tls.Config{
			Certificates:[]tls.Certificate{keyPair},
		}
	}

	return config, nil
}

func loadRawConfig(filename string) (map[string]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	rawconfig := make(map[string]string)
	scanner := bufio.NewScanner(file)

	comment := regexp.MustCompile(`^\s*#`)
	value := regexp.MustCompile(`^\s*(\S+)\s+(.*)$`)

	for scanner.Scan() {
		line := scanner.Text()

		if !comment.MatchString(line) {
			parts := value.FindStringSubmatch(line)

			if len(parts) == 3 {
				rawconfig[parts[1]] = parts[2]
			}
		}
	}

	return rawconfig, nil
}

func generateDefaultQuicConfig() *quic.Config {
	return &quic.Config{
		KeepAlive:          true,
		Versions:           []quic.VersionNumber{quic.VersionGQUIC43}, // Version 44 does not support multiplexing!
		ConnectionIDLength: connectionIdLength,
		IdleTimeout:        connectionIdleTimeout,
		HandshakeTimeout:   connectionHandshakeTimeout,
	}
}

func generateDefaultTLSServerConfig() *tls.Config {
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

	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
	}
}

func generateDefaultTLSClientConfig() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: true,
	}
}
