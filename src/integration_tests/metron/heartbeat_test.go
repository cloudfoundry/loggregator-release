package integration_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net"
	"github.com/cloudfoundry/sonde-go/control"
	"github.com/gogo/protobuf/proto"
)

var _ = Describe("Heartbeat", func() {

	It("sends heartbeat requests to clients", func(done Done) {
		defer close(done)
		listener, err := NewHeartbeatListener("localhost:51161")
		Expect(err).ToNot(HaveOccurred())
		originalMessage := basicValueMessage()

		err = listener.Write(originalMessage)
		Expect(err).ToNot(HaveOccurred())

		message, err := listener.ListenForHeartbeatRequest()
		Expect(err).ToNot(HaveOccurred())
		Expect(message.GetControlType()).To(Equal(control.ControlMessage_HeartbeatRequest))
	}, 2)
})

type heartbeatListener struct {
	udpAddr *net.UDPAddr
	udpConn net.PacketConn
}

func NewHeartbeatListener(remoteAddr string) (*heartbeatListener, error) {
	addr, err := net.ResolveUDPAddr("udp4", remoteAddr)
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenPacket("udp4", "")
	if err != nil {
		return nil, err
	}

	emitter := &heartbeatListener{udpAddr: addr, udpConn: conn}
	return emitter, nil
}

func (e *heartbeatListener) Write(data []byte) error {
	_, err := e.udpConn.WriteTo(data, e.udpAddr)
	return err
}


func (e *heartbeatListener) ListenForHeartbeatRequest() (*control.ControlMessage, error) {
	buf := make([]byte, 1024)
	n, _, err := e.udpConn.ReadFrom(buf)
	if err != nil {
		return nil, err
	}

	controlMessage := &control.ControlMessage{}
	err = proto.Unmarshal(buf[:n], controlMessage)
	if err != nil {
		return nil, err
	}
	return controlMessage, nil
}