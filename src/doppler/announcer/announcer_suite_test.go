package announcer_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"doppler/config"
	"github.com/pivotal-golang/localip"
	"testing"
)

func TestAnnouncer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Announcer Suite")
}

var (
	localIP string
	conf    config.Config
)

var _ = BeforeSuite(func() {
	localIP, _ = localip.LocalIP()
	conf = config.Config{
		JobName: "doppler_z1",
		Index:   0,
		EtcdMaxConcurrentRequests: 10,
		EtcdUrls:                  []string{"test:123", "test:456"},
		Zone:                      "z1",
		DropsondeIncomingMessagesPort: 1234,
	}
})
