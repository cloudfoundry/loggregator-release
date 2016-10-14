package lats_test

import (
	"os/exec"
	"testing"
	"time"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	latsConfig "lats/config"
	"lats/helpers"
)

var config *latsConfig.TestConfig

func TestLats(t *testing.T) {
	RegisterFailHandler(Fail)

	var metronSession *gexec.Session
	config = latsConfig.Load()

	BeforeSuite(func() {
		config.SaveMetronConfig()
		helpers.Initialize(config)
		metronSession = setupMetron()
	})

	AfterSuite(func() {
		metronSession.Kill().Wait()
	})

	RunSpecs(t, "Lats Suite")
}

func setupMetron() *gexec.Session {
	pathToMetronExecutable, err := gexec.Build("metron")
	Expect(err).ShouldNot(HaveOccurred())

	command := exec.Command(pathToMetronExecutable, "--config=fixtures/metron.json", "--debug")
	metronSession, err := gexec.Start(command, gexec.NewPrefixedWriter("[o][metron]", GinkgoWriter), gexec.NewPrefixedWriter("[e][metron]", GinkgoWriter))
	Expect(err).ShouldNot(HaveOccurred())

	Eventually(metronSession.Buffer).Should(gbytes.Say("Chose protocol"))
	Consistently(metronSession.Exited).ShouldNot(BeClosed())

	return metronSession
}

func createLogMessage(appID string) *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String(helpers.ORIGIN_NAME),
		EventType: events.Envelope_LogMessage.Enum(),
		Timestamp: proto.Int64(time.Now().UnixNano()),
		LogMessage: &events.LogMessage{
			Message:     []byte("test-log-message"),
			MessageType: events.LogMessage_OUT.Enum(),
			Timestamp:   proto.Int64(time.Now().UnixNano()),
			AppId:       proto.String(appID),
		},
	}
}

func createCounterEvent() *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String(helpers.ORIGIN_NAME),
		EventType: events.Envelope_CounterEvent.Enum(),
		Timestamp: proto.Int64(time.Now().UnixNano()),
		CounterEvent: &events.CounterEvent{
			Name:  proto.String("LATs-Counter"),
			Delta: proto.Uint64(5),
			Total: proto.Uint64(5),
		},
	}
}

func createValueMetric() *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String(helpers.ORIGIN_NAME),
		EventType: events.Envelope_ValueMetric.Enum(),
		Timestamp: proto.Int64(time.Now().UnixNano()),
		ValueMetric: &events.ValueMetric{
			Name:  proto.String("LATs-Value"),
			Value: proto.Float64(10),
			Unit:  proto.String("test-unit"),
		},
	}
}

func createContainerMetric(appId string) *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String(helpers.ORIGIN_NAME),
		EventType: events.Envelope_ContainerMetric.Enum(),
		Timestamp: proto.Int64(time.Now().UnixNano()),
		ContainerMetric: &events.ContainerMetric{
			ApplicationId: proto.String(appId),
			InstanceIndex: proto.Int32(1),
			CpuPercentage: proto.Float64(20.0),
			MemoryBytes:   proto.Uint64(10),
			DiskBytes:     proto.Uint64(20),
		},
	}
}
