package groupedsinks_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"net"
	"testing"
)

func TestGroupedsinks(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Groupedsinks Suite")
}

type fakeMessageWriter struct {
	RemoteAddress string
}

func (fake *fakeMessageWriter) RemoteAddr() net.Addr {
	return fakeAddr{remoteAddress: fake.RemoteAddress}
}

func (fake *fakeMessageWriter) WriteMessage(messageType int, data []byte) error {
	return nil
}

type fakeAddr struct {
	remoteAddress string
}

func (fake fakeAddr) Network() string {
	return "RemoteAddressNetwork"
}

func (fake fakeAddr) String() string {
	return fake.remoteAddress
}
