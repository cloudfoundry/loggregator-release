package experiment_test

import (
	"time"
	"tools/metronbenchmark/experiment"
	"tools/metronbenchmark/messagereader"
	"tools/metronbenchmark/messagewriter"

	"github.com/cloudfoundry/sonde-go/events"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var fakes []*fakeRound

var _ = Describe("Experiment", func() {
	var e *experiment.Experiment

	BeforeEach(func() {
		fakes = []*fakeRound{}

		c := make(chan *events.LogMessage)
		writer := messagewriter.NewChannelLogWriter(c)
		reader := messagereader.NewMessageReader(messagereader.NewChannelReader(c))
		go reader.Start()
		e = experiment.New(writer, reader, fakeFactory)
	})

	Describe("Start", func() {
		It("creates and executes a round", func() {
			stop := make(chan struct{})
			go e.Start(stop)

			Eventually(func() int { return len(fakes) }).Should(BeNumerically(">=", 1))
			Expect(fakes[0].performCalled).To(BeTrue())
			close(stop)
		})

		It("executes rounds in sequence with 10s duration and exponential rate", func() {
			stop := make(chan struct{})
			go e.Start(stop)

			Eventually(func() int { return len(fakes) }, 0.1, 5*time.Millisecond).Should(BeNumerically(">=", 3))
			close(stop)

			Expect(fakes[0].id).To(BeEquivalentTo(1))
			Expect(fakes[0].rate).To(BeEquivalentTo(100))
			Expect(fakes[0].duration).To(Equal(10 * time.Second))

			Expect(fakes[1].id).To(BeEquivalentTo(2))
			Expect(fakes[1].rate).To(BeEquivalentTo(200))
			Expect(fakes[1].duration).To(Equal(10 * time.Second))

			Expect(fakes[2].id).To(BeEquivalentTo(3))
			Expect(fakes[2].rate).To(BeEquivalentTo(400))
			Expect(fakes[2].duration).To(Equal(10 * time.Second))
		})

		It("stops when we close the stop channel", func() {
			stop := make(chan struct{})
			done := make(chan struct{})

			go func() {
				e.Start(stop)
				close(done)
			}()
			close(stop)

			Eventually(done).Should(BeClosed())
		})
	})

	Describe("Results", func() {
		It("builds the list of results", func() {
			stop := make(chan struct{})
			go e.Start(stop)

			Eventually(func() int { return len(fakes) }, 0.1, 5*time.Millisecond).Should(BeNumerically(">=", 3))
			close(stop)

			results := e.Results()
			Expect(len(results)).To(BeNumerically(">=", 3))

			Expect(results[0].MessageRate).To(BeEquivalentTo(100))
			Expect(results[0].MessagesSent).To(BeEquivalentTo(1))
			Expect(results[0].MessagesReceived).To(BeEquivalentTo(1))
			Expect(results[0].LossPercentage).To(BeEquivalentTo(0))

			Expect(results[1].MessageRate).To(BeEquivalentTo(200))
			Expect(results[1].MessagesSent).To(BeEquivalentTo(1))
			Expect(results[1].MessagesReceived).To(BeEquivalentTo(1))
			Expect(results[1].LossPercentage).To(BeEquivalentTo(0))

			Expect(results[2].MessageRate).To(BeEquivalentTo(400))
			Expect(results[2].MessagesSent).To(BeEquivalentTo(1))
			Expect(results[2].MessagesReceived).To(BeEquivalentTo(1))
			Expect(results[2].LossPercentage).To(BeEquivalentTo(0))
		})
	})
})

func fakeFactory(id uint, rate uint, d time.Duration) experiment.Round {
	fake := fakeRound{
		id:       id,
		rate:     rate,
		duration: d,
	}

	fakes = append(fakes, &fake)
	return &fake
}

type fakeRound struct {
	id       uint
	rate     uint
	duration time.Duration

	performCalled bool
}

func (r *fakeRound) Perform(writer experiment.MessageWriter) {
	r.performCalled = true
	writer.Send(r.id, time.Now())
	time.Sleep(time.Millisecond)
}

func (r *fakeRound) Id() uint {
	return r.id
}

func (r *fakeRound) Rate() uint {
	return r.rate
}

func (r *fakeRound) Duration() time.Duration {
	return r.duration
}
