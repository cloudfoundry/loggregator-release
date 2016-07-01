package forwarder_test

import (
	"boshhmforwarder/forwarder"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type SendValueMetricParamaters struct {
	deployment        string
	job               string
	index             string
	eventName         string
	secondsSinceEpoch int64
	value             float64
	unit              string
}

type fakeValueMetricSender struct {
	passedParametersCh chan *SendValueMetricParamaters
}

func newFakeValueMetricSender() *fakeValueMetricSender {
	return &fakeValueMetricSender{
		passedParametersCh: make(chan *SendValueMetricParamaters, 100),
	}
}

func (f *fakeValueMetricSender) SendValueMetric(deployment, job, index, eventName string, secondsSinceEpoch int64, value float64, unit string) error {
	parameters := &SendValueMetricParamaters{
		deployment:        deployment,
		job:               job,
		index:             index,
		eventName:         eventName,
		secondsSinceEpoch: secondsSinceEpoch,
		value:             value,
		unit:              unit,
	}
	f.passedParametersCh <- parameters

	return nil
}

var _ = Describe("MessageForwarder", func() {
	Context("Forwards the message", func() {
		var (
			fakeSender *fakeValueMetricSender
			messageCh  chan<- string
		)
		BeforeEach(func() {
			fakeSender = newFakeValueMetricSender()
			messageCh = forwarder.StartMessageForwarder(fakeSender)
		})

		It("should write messages to the sender", func(done Done) {
			defer close(done)
			messageCh <- "put system.healthy 1445463675 1 deployment=metrix-bosh-lite index=2 job=etcd role=unknown"
			var received *SendValueMetricParamaters
			Eventually(fakeSender.passedParametersCh).Should(Receive(&received))
			expectedParameters := SendValueMetricParamaters{
				deployment:        "metrix-bosh-lite",
				job:               "etcd",
				index:             "2",
				eventName:         "system.healthy",
				secondsSinceEpoch: 1445463675,
				value:             1.0,
				unit:              "b",
			}
			Expect(*received).Should(BeEquivalentTo(expectedParameters))
		}, 2)

		It("should write to Sender with 1m load with 'Load' for units", func(done Done) {
			defer close(done)
			messageCh <- "put system.load.1m 1445463675 1.59 deployment=metrix-bosh-lite index=0 job=etcd role=unknown"
			var received *SendValueMetricParamaters
			Eventually(fakeSender.passedParametersCh).Should(Receive(&received))
			expectedParameters := SendValueMetricParamaters{
				deployment:        "metrix-bosh-lite",
				job:               "etcd",
				index:             "0",
				eventName:         "system.load.1m",
				secondsSinceEpoch: 1445463675,
				value:             1.59,
				unit:              "Load",
			}
			Expect(*received).Should(BeEquivalentTo(expectedParameters))
		}, 2)

		It("should write to Sender with cpu user with 'Load' for units", func(done Done) {
			defer close(done)
			messageCh <- "put system.cpu.user 1445463675 3.8 deployment=metrix-bosh-lite index=0 job=etcd role=unknown"
			var received *SendValueMetricParamaters
			Eventually(fakeSender.passedParametersCh).Should(Receive(&received))
			expectedParameters := SendValueMetricParamaters{
				deployment:        "metrix-bosh-lite",
				job:               "etcd",
				index:             "0",
				eventName:         "system.cpu.user",
				secondsSinceEpoch: 1445463675,
				value:             3.8,
				unit:              "Load",
			}
			Expect(*received).Should(BeEquivalentTo(expectedParameters))
		}, 2)

		It("should write to Sender with cpu sys with 'Load' for units", func(done Done) {
			defer close(done)
			messageCh <- "put system.cpu.sys 1445463675 4.2 deployment=metrix-bosh-lite index=0 job=etcd role=unknown"
			var received *SendValueMetricParamaters
			Eventually(fakeSender.passedParametersCh).Should(Receive(&received))
			expectedParameters := SendValueMetricParamaters{
				deployment:        "metrix-bosh-lite",
				job:               "etcd",
				index:             "0",
				eventName:         "system.cpu.sys",
				secondsSinceEpoch: 1445463675,
				value:             4.2,
				unit:              "Load",
			}
			Expect(*received).Should(BeEquivalentTo(expectedParameters))
		}, 2)

		It("should write to Sender with cpu wait with 'Load' for units", func(done Done) {
			defer close(done)
			messageCh <- "put system.cpu.wait 1445463675 0.2 deployment=metrix-bosh-lite index=0 job=etcd role=unknown"
			var received *SendValueMetricParamaters
			Eventually(fakeSender.passedParametersCh).Should(Receive(&received))
			expectedParameters := SendValueMetricParamaters{
				deployment:        "metrix-bosh-lite",
				job:               "etcd",
				index:             "0",
				eventName:         "system.cpu.wait",
				secondsSinceEpoch: 1445463675,
				value:             0.2,
				unit:              "Load",
			}
			Expect(*received).Should(BeEquivalentTo(expectedParameters))
		}, 2)

		It("should write to Sender with disk system with 'Percent' for units", func(done Done) {
			defer close(done)
			messageCh <- "put system.disk.system.percent 1445463675 2 deployment=metrix-bosh-lite index=0 job=etcd role=unknown"
			var received *SendValueMetricParamaters
			Eventually(fakeSender.passedParametersCh).Should(Receive(&received))
			expectedParameters := SendValueMetricParamaters{
				deployment:        "metrix-bosh-lite",
				job:               "etcd",
				index:             "0",
				eventName:         "system.disk.system.percent",
				secondsSinceEpoch: 1445463675,
				value:             2.0,
				unit:              "Percent",
			}
			Expect(*received).Should(BeEquivalentTo(expectedParameters))
		}, 2)

		It("should write to Sender with system mem with 'Percent' for units", func(done Done) {
			defer close(done)
			messageCh <- "put system.mem.percent 1445463675 51 deployment=metrix-bosh-lite index=0 job=etcd role=unknown"
			var received *SendValueMetricParamaters
			Eventually(fakeSender.passedParametersCh).Should(Receive(&received))
			expectedParameters := SendValueMetricParamaters{
				deployment:        "metrix-bosh-lite",
				job:               "etcd",
				index:             "0",
				eventName:         "system.mem.percent",
				secondsSinceEpoch: 1445463675,
				value:             51.0,
				unit:              "Percent",
			}
			Expect(*received).Should(BeEquivalentTo(expectedParameters))
		}, 2)

		It("should write to Sender with system swap with 'Percent' for units", func(done Done) {
			defer close(done)
			messageCh <- "put system.swap.percent 1445463675 41 deployment=metrix-bosh-lite index=0 job=etcd role=unknown"
			var received *SendValueMetricParamaters
			Eventually(fakeSender.passedParametersCh).Should(Receive(&received))
			expectedParameters := SendValueMetricParamaters{
				deployment:        "metrix-bosh-lite",
				job:               "etcd",
				index:             "0",
				eventName:         "system.swap.percent",
				secondsSinceEpoch: 1445463675,
				value:             41.0,
				unit:              "Percent",
			}
			Expect(*received).Should(BeEquivalentTo(expectedParameters))
		}, 2)

		It("should write to Sender with system mem with 'Kb' for units", func(done Done) {
			defer close(done)
			messageCh <- "put system.mem.kb 1445463675 5224880 deployment=metrix-bosh-lite index=0 job=etcd role=unknown"
			var received *SendValueMetricParamaters
			Eventually(fakeSender.passedParametersCh).Should(Receive(&received))
			expectedParameters := SendValueMetricParamaters{
				deployment:        "metrix-bosh-lite",
				job:               "etcd",
				index:             "0",
				eventName:         "system.mem.kb",
				secondsSinceEpoch: 1445463675,
				value:             5224880.0,
				unit:              "Kb",
			}
			Expect(*received).Should(BeEquivalentTo(expectedParameters))
		}, 2)

		It("should write to Sender with system swap with 'Kb' for units", func(done Done) {
			defer close(done)
			messageCh <- "put system.swap.kb 1445463675 431484 deployment=metrix-bosh-lite index=0 job=etcd role=unknown"
			var received *SendValueMetricParamaters
			Eventually(fakeSender.passedParametersCh).Should(Receive(&received))
			expectedParameters := SendValueMetricParamaters{
				deployment:        "metrix-bosh-lite",
				job:               "etcd",
				index:             "0",
				eventName:         "system.swap.kb",
				secondsSinceEpoch: 1445463675,
				value:             431484.0,
				unit:              "Kb",
			}
			Expect(*received).Should(BeEquivalentTo(expectedParameters))
		}, 2)

		It("should handle failure for missing fields for partial message", func(done Done) {
			defer close(done)
			messageCh <- "put system.healthy 1445463675 1"
			var received *SendValueMetricParamaters
			Eventually(fakeSender.passedParametersCh).Should(Receive(&received))
			expectedParameters := SendValueMetricParamaters{
				deployment:        "",
				job:               "",
				index:             "",
				eventName:         "system.healthy",
				secondsSinceEpoch: 1445463675,
				value:             1.0,
				unit:              "b",
			}
			Expect(*received).Should(BeEquivalentTo(expectedParameters))
		}, 1)

		It("should handle multiple messages", func(done Done) {
			defer close(done)
			messageCh <- "put system.disk.system.percent 1445463675 2 deployment=metrix-bosh-lite index=0 job=etcd role=unknown"
			messageCh <- "put system.healthy 1445463675 1 deployment=metrix-bosh-lite index=2 job=etcd role=unknown"
			messageCh <- "put system.load.1m 1445463675 1.59 deployment=metrix-bosh-lite index=0 job=etcd role=unknown"
			messageCh <- "put system.cpu.user 1445463675 3.8 deployment=metrix-bosh-lite index=0 job=etcd role=unknown"
			messageCh <- "put system.cpu.sys 1445463675 4.2 deployment=metrix-bosh-lite index=0 job=etcd role=unknown"
			messageCh <- "put system.cpu.wait 1445463675 0.2 deployment=metrix-bosh-lite index=0 job=etcd role=unknown"
			messageCh <- "put system.mem.percent 1445463675 51 deployment=metrix-bosh-lite index=0 job=etcd role=unknown"
			messageCh <- "put system.mem.kb 1445463675 5224880 deployment=metrix-bosh-lite index=0 job=etcd role=unknown"
			messageCh <- "put system.swap.percent 1445463675 41 deployment=metrix-bosh-lite index=0 job=etcd role=unknown"
			messageCh <- "put system.swap.kb 1445463675 431484 deployment=metrix-bosh-lite index=0 job=etcd role=unknown"
			Eventually(fakeSender.passedParametersCh).Should(HaveLen(10))
		}, 2)

		It("extra fields should not cause a failure", func(done Done) {
			defer close(done)
			messageCh <- "put system.cpu.sys 1445463675 4.2 deployment=metrix-bosh-lite index=0 job=etcd role=unknown something=new"
			var received *SendValueMetricParamaters
			Eventually(fakeSender.passedParametersCh).Should(Receive(&received))
			expectedParameters := SendValueMetricParamaters{
				deployment:        "metrix-bosh-lite",
				job:               "etcd",
				index:             "0",
				eventName:         "system.cpu.sys",
				secondsSinceEpoch: 1445463675,
				value:             4.2,
				unit:              "Load",
			}
			Expect(*received).Should(BeEquivalentTo(expectedParameters))
		}, 1)

		It("should not forward invalid data", func() {
			messageCh <- "This is a bad message without any real data"
			Consistently(fakeSender.passedParametersCh).ShouldNot(Receive())
		})

		It("Key value pairs can be in any order", func(done Done) {
			defer close(done)
			messageCh <- "put system.cpu.sys 1445463675 4.2 role=unknown index=0 job=etcd deployment=metrix-bosh-lite"
			var received *SendValueMetricParamaters
			Eventually(fakeSender.passedParametersCh).Should(Receive(&received))
			expectedParameters := SendValueMetricParamaters{
				deployment:        "metrix-bosh-lite",
				job:               "etcd",
				index:             "0",
				eventName:         "system.cpu.sys",
				secondsSinceEpoch: 1445463675,
				value:             4.2,
				unit:              "Load",
			}
			Expect(*received).Should(BeEquivalentTo(expectedParameters))
		}, 2)

		It("Missing Key value pair should not cause a failure", func(done Done) {
			defer close(done)
			messageCh <- "put system.cpu.sys 1445463675 4.2 role=unknown instance=0 job=etcd deployment=metrix-bosh-lite"
			var received *SendValueMetricParamaters
			Eventually(fakeSender.passedParametersCh).Should(Receive(&received))
			expectedParameters := SendValueMetricParamaters{
				deployment:        "metrix-bosh-lite",
				job:               "etcd",
				index:             "",
				eventName:         "system.cpu.sys",
				secondsSinceEpoch: 1445463675,
				value:             4.2,
				unit:              "Load",
			}
			Expect(*received).Should(BeEquivalentTo(expectedParameters))
		}, 1)
	})
})
