package experiment_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"net"
	"testing"
)

func TestExperiment(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Experiment Suite")
}

var fakeMetron net.PacketConn
var _ = BeforeSuite(func() {
	var err error
	fakeMetron, err = net.ListenPacket("udp4", ":0")
	if err != nil {
		panic(err)
	}
})

var _ = AfterSuite(func() {
	fakeMetron.Close()
})
