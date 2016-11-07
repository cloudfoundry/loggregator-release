package groupedsinks_test

import (
	"log"
	"net"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestGroupedsinks(t *testing.T) {
	log.SetOutput(GinkgoWriter)
	RegisterFailHandler(Fail)
	RunSpecs(t, "GroupedSinks Suite")
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

func (fake *fakeMessageWriter) SetWriteDeadline(t time.Time) error {
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
