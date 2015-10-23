package metrics_test

import (
	"integration_tests/runners"
	"io/ioutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"os"
	"testing"

	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/gogo/protobuf/proto"
	"github.com/nu7hatch/gouuid"
	"github.com/onsi/ginkgo/config"
	"github.com/onsi/gomega/gexec"
)

var tmpdir string
var etcdRunner *etcdstorerunner.ETCDClusterRunner
var etcdAdapter storeadapter.StoreAdapter
var port int
var pathToMetronExecutable string
var metronRunner *runners.MetronRunner

func TestMetrics(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metrics Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	metronPath, err := gexec.Build("metron", "-race")
	Expect(err).ShouldNot(HaveOccurred())
	return []byte(metronPath)
}, func(path []byte) {
	metronPath := string(path)

	var err error
	tmpdir, err = ioutil.TempDir("", "metronmetrics")
	Expect(err).ShouldNot(HaveOccurred())

	etcdPort := 5800 + (config.GinkgoConfig.ParallelNode)*10
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1, nil)
	etcdRunner.Start()
	etcdAdapter = etcdRunner.Adapter(nil)

	port := 51000 + config.GinkgoConfig.ParallelNode*10
	metronRunner = &runners.MetronRunner{
		Path:          metronPath,
		TempDir:       tmpdir,
		LegacyPort:    port,
		MetronPort:    port + 1,
		DropsondePort: 3457 + config.GinkgoConfig.ParallelNode*10,
		EtcdRunner:    etcdRunner,
	}
})

var _ = SynchronizedAfterSuite(func() {
	if etcdRunner != nil {
		etcdRunner.Stop()
	}
	os.RemoveAll(tmpdir)
}, func() {
	gexec.CleanupBuildArtifacts()
})

var _ = BeforeEach(func() {
	etcdRunner.Reset()
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
