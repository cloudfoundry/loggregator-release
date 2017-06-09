package lats_test

import (
	"fmt"
	"math/rand"
	"sort"
	"time"

	"code.cloudfoundry.org/loggregator/lats/helpers"

	"github.com/cloudfoundry/noaa/consumer"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Firehose", func() {

	var generateSubID = func() string {
		rand.Seed(time.Now().UnixNano())
		return fmt.Sprintf("sub-%d", rand.Int())
	}

	var buildValueMetric = func(name string, value float64) *events.Envelope {
		valueMetric := createValueMetric()
		valueMetric.ValueMetric.Name = proto.String(name)
		valueMetric.ValueMetric.Value = proto.Float64(value)
		return valueMetric
	}

	var emitControlMessages = func() {
		for i := 0; i < 20; i++ {
			time.Sleep(10 * time.Millisecond)
			helpers.EmitToMetronV1(buildValueMetric("controlValue", 0))
		}
	}

	var waitForControl = func(msgs <-chan *events.Envelope) {
		Eventually(msgs).Should(Receive())
	}

	var readFromErrors = func(errs <-chan error) {
		defer GinkgoRecover()
		Consistently(errs).ShouldNot(Receive())
	}

	var readEnvelopes = func(t time.Duration, msgs <-chan *events.Envelope) []*events.Envelope {
		var envelopes []*events.Envelope
		timer := time.NewTimer(t)
		for {
			select {
			case <-timer.C:
				return envelopes
			case e := <-msgs:
				if e.GetOrigin() == helpers.OriginName && e.ValueMetric.GetName() == "mainValue" {
					envelopes = append(envelopes, e)
				}
			}
		}
	}

	var verifyEnvelopes = func(count int, envelopes []*events.Envelope) bool {
		sort.Sort(valueMetrics(envelopes))
		var i float64
		for _, e := range envelopes {
			if e.ValueMetric.GetValue() < i {
				return false
			}
			i = e.ValueMetric.GetValue() + 1
		}

		return true
	}

	var emitMetrics = func(count int) {
		for i := 0; i < count; i++ {
			time.Sleep(10 * time.Millisecond)
			helpers.EmitToMetronV1(buildValueMetric("mainValue", float64(i)))
		}
	}

	Describe("subscription sharding", func() {
		Context("100 envelopes emitted", func() {
			var (
				count int
			)

			BeforeEach(func() {
				count = 100
			})

			JustBeforeEach(func() {
				for i := 0; i < count; i++ {
					time.Sleep(10 * time.Millisecond)
					helpers.EmitToMetronV1(buildValueMetric("mainValue", float64(i)))
				}
			})

			Context("single connection", func() {
				var (
					reader *consumer.Consumer
					msgs   <-chan *events.Envelope
					errs   <-chan error
				)

				JustBeforeEach(func() {
					go emitControlMessages()
					waitForControl(msgs)
				})

				BeforeEach(func() {
					reader, _ = helpers.SetUpConsumer()
					msgs, errs = reader.Firehose(generateSubID(), "")
					go readFromErrors(errs)
				})

				AfterEach(func() {
					reader.Close()
				})

				It("sends all the envelopes to the subscription", func() {
					envelopes := readEnvelopes(2*time.Second, msgs)

					Expect(len(envelopes)).To(BeNumerically("~", count, 3))
					Expect(len(envelopes)).To(BeNumerically("<=", count))
					Expect(verifyEnvelopes(count, envelopes)).To(BeTrue())
				})
			})

			Context("multiple connections", func() {
				var (
					reader *consumer.Consumer
					msgs1  <-chan *events.Envelope
					msgs2  <-chan *events.Envelope
					errs1  <-chan error
					errs2  <-chan error
				)

				AfterEach(func() {
					reader.Close()
				})

				var readFromTwoChannels = func() ([]*events.Envelope, []*events.Envelope) {
					var envelopes1, envelopes2 []*events.Envelope
					t := time.NewTimer(5 * time.Second)
					for {
						select {
						case <-t.C:
							return envelopes1, envelopes2
						case e := <-msgs1:
							if e.GetOrigin() == helpers.OriginName && e.ValueMetric.GetName() == "mainValue" {
								envelopes1 = append(envelopes1, e)
							}
						case e := <-msgs2:
							if e.GetOrigin() == helpers.OriginName && e.ValueMetric.GetName() == "mainValue" {
								envelopes2 = append(envelopes2, e)
							}
						}
					}
				}

				Context("single subscription ID", func() {
					BeforeEach(func() {
						subID := generateSubID()
						reader, _ = helpers.SetUpConsumer()
						msgs1, errs1 = reader.Firehose(subID, "")
						msgs2, errs2 = reader.Firehose(subID, "")
						go readFromErrors(errs1)
						go readFromErrors(errs2)
					})

					JustBeforeEach(func() {
						go emitControlMessages()
						waitForControl(msgs1)
						waitForControl(msgs2)
					})

					It("sends about half of the envelopes to each connection", func() {
						envelopes1, envelopes2 := readFromTwoChannels()

						Expect(len(envelopes1)).To(BeNumerically("~", count/2, 15))
						Expect(len(envelopes2)).To(BeNumerically("~", count/2, 15))

						allEnvelopes := append(envelopes1, envelopes2...)
						Expect(verifyEnvelopes(count, allEnvelopes)).To(BeTrue())
					})
				})

				Context("2 subscription IDs", func() {
					Context("1 connection per subscription", func() {
						BeforeEach(func() {
							subID1 := generateSubID()
							subID2 := generateSubID()
							reader, _ = helpers.SetUpConsumer()
							msgs1, errs1 = reader.Firehose(subID1, "")
							msgs2, errs2 = reader.Firehose(subID2, "")
							go readFromErrors(errs1)
							go readFromErrors(errs2)
						})

						JustBeforeEach(func() {
							go emitControlMessages()
							waitForControl(msgs1)
							waitForControl(msgs2)
						})

						It("sends all the envelopes to each connection", func() {
							envelopes1, envelopes2 := readFromTwoChannels()

							Expect(len(envelopes1)).To(BeNumerically("~", count, 3))
							Expect(len(envelopes1)).To(BeNumerically("<=", count))

							Expect(len(envelopes2)).To(BeNumerically("~", count, 3))
							Expect(len(envelopes2)).To(BeNumerically("<=", count))

							Expect(verifyEnvelopes(count, envelopes1)).To(BeTrue())
							Expect(verifyEnvelopes(count, envelopes2)).To(BeTrue())
						})
					})

					Context("2 connections per subscription", func() {
						var (
							// Subscription A
							msgsA1 <-chan *events.Envelope
							msgsA2 <-chan *events.Envelope
							errsA1 <-chan error
							errsA2 <-chan error

							// Subscription B
							msgsB1 <-chan *events.Envelope
							msgsB2 <-chan *events.Envelope
							errsB1 <-chan error
							errsB2 <-chan error
						)

						var readFromFourChannels = func() [][]*events.Envelope {
							var envelopesA1, envelopesA2, envelopesB1, envelopesB2 []*events.Envelope
							t := time.NewTimer(5 * time.Second)
							for {
								select {
								case <-t.C:
									return [][]*events.Envelope{
										envelopesA1,
										envelopesA2,
										envelopesB1,
										envelopesB2,
									}
								case e := <-msgsA1:
									if e.GetOrigin() == helpers.OriginName && e.ValueMetric.GetName() == "mainValue" {
										envelopesA1 = append(envelopesA1, e)
									}
								case e := <-msgsA2:
									if e.GetOrigin() == helpers.OriginName && e.ValueMetric.GetName() == "mainValue" {
										envelopesA2 = append(envelopesA2, e)
									}

								case e := <-msgsB1:
									if e.GetOrigin() == helpers.OriginName && e.ValueMetric.GetName() == "mainValue" {
										envelopesB1 = append(envelopesB1, e)
									}
								case e := <-msgsB2:
									if e.GetOrigin() == helpers.OriginName && e.ValueMetric.GetName() == "mainValue" {
										envelopesB2 = append(envelopesB2, e)
									}
								}
							}
						}

						BeforeEach(func() {
							reader, _ = helpers.SetUpConsumer()

							subIDa := generateSubID()
							msgsA1, errsA1 = reader.Firehose(subIDa, "")
							msgsA2, errsA2 = reader.Firehose(subIDa, "")
							go readFromErrors(errsA1)
							go readFromErrors(errsA2)

							subIDb := generateSubID()
							msgsB1, errsB1 = reader.Firehose(subIDb, "")
							msgsB2, errsB2 = reader.Firehose(subIDb, "")
							go readFromErrors(errsB1)
							go readFromErrors(errsB2)
						})

						JustBeforeEach(func() {
							go emitControlMessages()

							waitForControl(msgsA1)
							waitForControl(msgsA2)

							waitForControl(msgsB1)
							waitForControl(msgsB2)
						})

						It("sends about half of the envelopes to each connection per subscription", func() {
							envelopes := readFromFourChannels()

							By("subscription A")
							Expect(len(envelopes[0])).To(BeNumerically("~", count/2, 15))
							Expect(len(envelopes[1])).To(BeNumerically("~", count/2, 15))

							By("subscription B")
							Expect(len(envelopes[2])).To(BeNumerically("~", count/2, 15))
							Expect(len(envelopes[3])).To(BeNumerically("~", count/2, 15))

							By("subscription A")
							allEnvelopesA := append(envelopes[0], envelopes[1]...)
							Expect(verifyEnvelopes(count, allEnvelopesA)).To(BeTrue())

							By("subscription B")
							allEnvelopesB := append(envelopes[2], envelopes[3]...)
							Expect(verifyEnvelopes(count, allEnvelopesB)).To(BeTrue())
						})
					})
				})

			})
		})
	})

	Describe("subscription reconnect", func() {
		It("gets all of the messages after reconnect", func() {
			By("establishing first consumer")
			consumer, _ := helpers.SetUpConsumer()
			subscriptionID := generateSubID()
			msgs, errs := consumer.FirehoseWithoutReconnect(subscriptionID, "")
			go readFromErrors(errs)

			By("waiting for connection to be established")
			emitControlMessages()
			waitForControl(msgs)
			consumer.Close()

			By("establishing second consumer with the same subscription id")
			consumer, _ = helpers.SetUpConsumer()
			msgs, errs = consumer.FirehoseWithoutReconnect(subscriptionID, "")
			go readFromErrors(errs)

			By("asserting that most messages get through")
			count := 100
			emitMetrics(count)
			envelopes := readEnvelopes(2*time.Second, msgs)

			Expect(len(envelopes)).To(BeNumerically("~", count, 3))
			Expect(len(envelopes)).To(BeNumerically("<=", count))
			Expect(verifyEnvelopes(count, envelopes)).To(BeTrue())
		})
	})
})

type valueMetrics []*events.Envelope

func (m valueMetrics) Len() int {
	return len(m)
}

func (m valueMetrics) Less(i, j int) bool {
	return m[i].ValueMetric.GetValue() < m[j].ValueMetric.GetValue()
}

func (m valueMetrics) Swap(i, j int) {
	tmp := m[i]
	m[i] = m[j]
	m[j] = tmp
}
