package tlslistener_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"crypto/tls"
	"io/ioutil"
	"testing"
)

func TestTcplistener(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tcplistener Suite")
}

var config *tls.Config

var _ = BeforeSuite(func() {
	certContent, err := ioutil.ReadFile("fixtures/key.crt")
	if err != nil {
		panic(err)
	}

	keyContent, err := ioutil.ReadFile("fixtures/key.key")
	if err != nil {
		panic(err)
	}

	cert, err := tls.X509KeyPair(certContent, keyContent)
	if err != nil {
		panic(err)
	}

	config = &tls.Config{
		InsecureSkipVerify:     true,
		Certificates:           []tls.Certificate{cert},
		SessionTicketsDisabled: true,
	}
})
