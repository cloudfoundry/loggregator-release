package lats_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	latsConfig "code.cloudfoundry.org/loggregator/lats/config"
	"code.cloudfoundry.org/loggregator/lats/helpers"
)

var config *latsConfig.TestConfig

func TestLats(t *testing.T) {
	RegisterFailHandler(Fail)

	config = latsConfig.Load()

	BeforeSuite(func() {
		config.SaveMetronConfig()
		helpers.Initialize(config)
		setupMetron()
	})

	RunSpecs(t, "Lats Suite")
}

func setupMetron() {
	time.Sleep(10 * time.Second)
}

func createCounterEvent() *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String(helpers.OriginName),
		EventType: events.Envelope_CounterEvent.Enum(),
		Timestamp: proto.Int64(time.Now().UnixNano()),
		CounterEvent: &events.CounterEvent{
			Name:  proto.String(fmt.Sprintf("LATs-Counter-%d", time.Now().UnixNano())),
			Delta: proto.Uint64(5),
			Total: proto.Uint64(5),
		},
		Tags: map[string]string{"UniqueName": fmt.Sprintf("LATs-%d", time.Now().UnixNano())},
	}
}

func createValueMetric() *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String(helpers.OriginName),
		EventType: events.Envelope_ValueMetric.Enum(),
		Timestamp: proto.Int64(time.Now().UnixNano()),
		ValueMetric: &events.ValueMetric{
			Name:  proto.String("LATs-Value"),
			Value: proto.Float64(10),
			Unit:  proto.String("test-unit"),
		},
		Tags: map[string]string{"UniqueName": fmt.Sprintf("LATs-%d", time.Now().UnixNano())},
	}
}

func createContainerMetric(appId string) *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String(helpers.OriginName),
		EventType: events.Envelope_ContainerMetric.Enum(),
		Timestamp: proto.Int64(time.Now().UnixNano()),
		ContainerMetric: &events.ContainerMetric{
			ApplicationId: proto.String(appId),
			InstanceIndex: proto.Int32(1),
			CpuPercentage: proto.Float64(20.0),
			MemoryBytes:   proto.Uint64(10),
			DiskBytes:     proto.Uint64(20),
		},
		Tags: map[string]string{"UniqueName": fmt.Sprintf("LATs-%d", time.Now().UnixNano())},
	}
}
