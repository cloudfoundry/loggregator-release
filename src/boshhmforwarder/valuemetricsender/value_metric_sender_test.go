package valuemetricsender_test

import (
	"boshhmforwarder/valuemetricsender"

	"time"

	"github.com/cloudfoundry/dropsonde/emitter/fake"
	"github.com/cloudfoundry/dropsonde/envelope_sender"
	"github.com/cloudfoundry/dropsonde/envelopes"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ValueMetricSender", func() {
	var (
		fakeEventEmitter *fake.FakeEventEmitter
	)
	BeforeEach(func() {
		fakeEventEmitter = fake.NewFakeEventEmitter("MonitorTest")
		envelopes.Initialize(envelope_sender.NewEnvelopeSender(fakeEventEmitter))
	})

	AfterEach(func() {
		fakeEventEmitter.Close()
	})

	It("emits bosh metrics", func() {
		valueMetricSender := valuemetricsender.NewValueMetricSender()
		timeInSeconds := time.Now().Unix()
		valueMetricSender.SendValueMetric("some-deployment", "some-job", "some-index", "some-event-name", timeInSeconds, 1.23, "some-unit")

		Eventually(fakeEventEmitter.GetEnvelopes()).Should(HaveLen(1))
		Expect(fakeEventEmitter.GetEnvelopes()[0].Origin).Should(Equal(proto.String(valuemetricsender.ForwarderOrigin)))
		Expect(fakeEventEmitter.GetEnvelopes()[0].ValueMetric).ShouldNot(BeNil())
		Expect(fakeEventEmitter.GetEnvelopes()[0].Timestamp).ShouldNot(Equal(timeInSeconds * int64(time.Second)))
	})
})
