package ingress_test

import (
	"errors"

	"code.cloudfoundry.org/loggregator/plumbing/conversion"

	"code.cloudfoundry.org/loggregator/rlp/internal/ingress"

	"golang.org/x/net/context"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Querier", func() {
	var (
		spyFetcher   *spyContainerMetricFetcher
		spyConverter *SpyEnvelopeConverter
		server       *ingress.Querier
	)

	BeforeEach(func() {
		spyFetcher = newSpyContainerMetricFetcher()
		spyConverter = &SpyEnvelopeConverter{}
		server = ingress.NewQuerier(spyConverter, spyFetcher)
	})

	It("requests the correct appID", func() {
		ctx := context.TODO()
		server.ContainerMetrics(ctx, "some-app", false)
		Expect(spyFetcher.appID).To(Equal("some-app"))
		Expect(spyFetcher.ctx).To(Equal(ctx))
	})

	It("use envelope converter", func() {
		v1Env, v1Data := buildContainerMetric()
		v2Env := conversion.ToV2(v1Env, false)
		spyFetcher.results = [][]byte{v1Data}
		spyConverter.envelope = v2Env

		results, err := server.ContainerMetrics(context.TODO(), "some-app", false)

		Expect(err).ToNot(HaveOccurred())
		Expect(results).To(HaveLen(1))
		Expect(results[0]).To(Equal(v2Env))
		Expect(spyConverter.data).To(Equal(v1Data))
	})

	It("converts using preferred tags", func() {
		spyFetcher.results = [][]byte{nil}

		server.ContainerMetrics(context.TODO(), "some-app", true)
		Expect(spyConverter.usePreferredTags).To(BeTrue())
	})

	It("skips envelopes that do not convert", func() {
		spyConverter.err = errors.New("some-error")
		results, err := server.ContainerMetrics(context.TODO(), "some-app", false)
		Expect(err).ToNot(HaveOccurred())
		Expect(results).To(HaveLen(0))
	})
})

type spyContainerMetricFetcher struct {
	results [][]byte
	appID   string
	ctx     context.Context
}

func newSpyContainerMetricFetcher() *spyContainerMetricFetcher {
	return &spyContainerMetricFetcher{}
}

func (s *spyContainerMetricFetcher) ContainerMetrics(ctx context.Context, appID string) [][]byte {
	s.ctx = ctx
	s.appID = appID
	return s.results
}

func buildContainerMetric() (*events.Envelope, []byte) {
	e := &events.Envelope{
		Origin:    proto.String("some-origin"),
		EventType: events.Envelope_ContainerMetric.Enum(),
		ContainerMetric: &events.ContainerMetric{
			ApplicationId: proto.String("test-app"),
			InstanceIndex: proto.Int32(1),
			CpuPercentage: proto.Float64(10.0),
			MemoryBytes:   proto.Uint64(1),
			DiskBytes:     proto.Uint64(1),
		},
	}
	b, err := proto.Marshal(e)
	if err != nil {
		panic(err)
	}
	return e, b
}
