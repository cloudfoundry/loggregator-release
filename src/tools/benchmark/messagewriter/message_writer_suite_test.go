package messagewriter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"net"
	"testing"
)

func TestMessagewriter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Messagewriter Suite")

}

var fakeMetron net.PacketConn
var _ = BeforeSuite(func() {
	var err error
	fakeMetron, err = net.ListenPacket("udp4", ":51161")
	if err != nil {
		panic(err)
	}
})

var _ = AfterSuite(func() {
	fakeMetron.Close()
})
