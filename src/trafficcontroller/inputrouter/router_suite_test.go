package inputrouter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"net"
	"testing"
	"time"
)

func TestInputRouter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "InputRouter Suite")
}

var verifyListenerStarted = func(listenerChan <-chan []byte, listenerPort string) {
	connection, _ := net.Dial("udp", "localhost:"+listenerPort)
	connection.Write([]byte("test-data"))
	connection.Close()

	for counter := 0; counter < 5; counter++ {
		select {
		case <-listenerChan:
			return
		case <-time.After(time.Second):
			connection, _ := net.Dial("udp", "localhost:"+listenerPort)
			connection.Write([]byte("test-data"))
			connection.Close()
		}
	}

	panic("Could not set up connection")
}
