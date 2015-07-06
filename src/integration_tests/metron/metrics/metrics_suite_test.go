package metrics_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"net/http"
	"os/exec"
	"testing"

	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/gogo/protobuf/proto"
	"github.com/nu7hatch/gouuid"
	"github.com/onsi/ginkgo/config"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-golang/localip"
)

var metronSession *gexec.Session
var etcdRunner *etcdstorerunner.ETCDClusterRunner
var etcdPort int
var localIPAddress string
var pathToMetronExecutable string

func TestMetrics(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metrics Suite")
}

var _ = BeforeSuite(func() {
	var err error
	pathToMetronExecutable, err = gexec.Build("metron")
	Expect(err).ShouldNot(HaveOccurred())

	localIPAddress, _ = localip.LocalIP()

	etcdPort = 5800 + (config.GinkgoConfig.ParallelNode-1)*10
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1)
	etcdRunner.Start()
})

var _ = BeforeEach(func() {
	var err error
	command := exec.Command(pathToMetronExecutable, "--config=fixtures/metron.json", "--debug")
	metronSession, err = gexec.Start(command, gexec.NewPrefixedWriter("[o][metron]", GinkgoWriter), gexec.NewPrefixedWriter("[e][metron]", GinkgoWriter))
	Expect(err).ShouldNot(HaveOccurred())

	// wait for server to be up
	Eventually(func() error {
		_, err := http.Get("http://" + localIPAddress + ":1234")
		return err
	}, 3).ShouldNot(HaveOccurred())
})

var _ = AfterEach(func() {
	metronSession.Kill().Wait()
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()

	etcdRunner.Adapter().Disconnect()
	etcdRunner.Stop()
})

func basicValueMessage() []byte {
	message, _ := proto.Marshal(basicValueMessageEnvelope())
	return message
}

func basicValueMessageEnvelope() *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String("fake-origin-2"),
		EventType: events.Envelope_ValueMetric.Enum(),
		ValueMetric: &events.ValueMetric{
			Name:  proto.String("fake-metric-name"),
			Value: proto.Float64(42),
			Unit:  proto.String("fake-unit"),
		},
	}
}

func basicCounterEvent() []byte {
	message, err := proto.Marshal(basicCounterEventEnvelope())
	if err != nil {
		panic(err)
	}
	return message
}

func basicCounterEventEnvelope() *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String("fake-origin-2"),
		EventType: events.Envelope_CounterEvent.Enum(),
		CounterEvent: &events.CounterEvent{
			Name:  proto.String("fake-metric-name"),
			Delta: proto.Uint64(3),
			Total: proto.Uint64(3),
		},
	}
}

func basicHTTPStartEvent() []byte {
	message, err := proto.Marshal(basicHTTPStartEventEnvelope())
	if err != nil {
		panic(err)
	}
	return message
}

func basicHTTPStartEventEnvelope() *events.Envelope {
	uuid, _ := uuid.ParseHex("9f45fa9d-dbf8-463e-425f-a79d30e1b56f")
	return &events.Envelope{
		Origin:    proto.String("fake-origin-2"),
		EventType: events.Envelope_HttpStart.Enum(),
		HttpStart: &events.HttpStart{
			Timestamp:     proto.Int64(12),
			RequestId:     factories.NewUUID(uuid),
			PeerType:      events.PeerType_Client.Enum(),
			Method:        events.Method_GET.Enum(),
			Uri:           proto.String("some uri"),
			RemoteAddress: proto.String("some address"),
			UserAgent:     proto.String("some user agent"),
		},
	}
}

func basicHTTPStopEvent() []byte {
	message, err := proto.Marshal(basicHTTPStopEventEnvelope())
	if err != nil {
		panic(err)
	}
	return message
}

func basicHTTPStopEventEnvelope() *events.Envelope {
	uuid, _ := uuid.ParseHex("9f45fa9d-dbf8-463e-425f-a79d30e1b56f")
	return &events.Envelope{
		Origin:    proto.String("fake-origin-2"),
		EventType: events.Envelope_HttpStop.Enum(),
		HttpStop: &events.HttpStop{
			Timestamp:     proto.Int64(12),
			Uri:           proto.String("some uri"),
			RequestId:     factories.NewUUID(uuid),
			PeerType:      events.PeerType_Client.Enum(),
			StatusCode:    proto.Int32(404),
			ContentLength: proto.Int64(98475189),
		},
	}
}
