package sinkmanager_test

import (
	"net"
	"net/url"
	"sync"
	"time"

	"code.cloudfoundry.org/loggregator/doppler/internal/iprange"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinks"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinks/dump"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinks/syslog"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinks/syslogwriter"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinkserver/blacklist"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinkserver/sinkmanager"
	"code.cloudfoundry.org/loggregator/doppler/internal/store"
	"code.cloudfoundry.org/loggregator/metricemitter/testhelper"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SinkManager", func() {
	var blackListManager = blacklist.New([]iprange.IPRange{{Start: "10.10.10.10", End: "10.10.10.20"}})
	var sinkManager *sinkmanager.SinkManager
	var sinkManagerDone chan struct{}
	var newAppServiceChan, deletedAppServiceChan chan store.AppService

	BeforeEach(func() {
		fakeMetricSender.Reset()

		health := newSpyHealthRegistrar()
		sinkManager = sinkmanager.New(1, true, blackListManager, 100,
			"dropsonde-origin", 1*time.Second, 0, 1*time.Second,
			1*time.Second, nil, testhelper.NewMetricClient(), health)

		newAppServiceChan = make(chan store.AppService)
		deletedAppServiceChan = make(chan store.AppService)

		sinkManagerDone = make(chan struct{})
		go func() {
			defer close(sinkManagerDone)
			sinkManager.Start(newAppServiceChan, deletedAppServiceChan)
		}()
	})

	AfterEach(func() {
		sinkManager.Stop()
		<-sinkManagerDone
	})

	Describe("SendTo", func() {
		It("sends to all known sinks", func() {
			sink1 := &channelSink{appId: "myApp",
				identifier: "myAppChan1",
				done:       make(chan struct{}),
			}
			sink2 := &channelSink{appId: "myApp",
				identifier: "myAppChan2",
				done:       make(chan struct{}),
			}

			sinkManager.RegisterSink(sink1)
			sinkManager.RegisterSink(sink2)

			expectedMessageString := "Some Data"
			expectedMessage, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, expectedMessageString, "myApp", "App"), "origin")
			go sinkManager.SendTo("myApp", expectedMessage)

			Eventually(sink1.Received).Should(HaveLen(1))
			Eventually(sink2.Received).Should(HaveLen(1))
			Expect(sink1.Received()[0]).To(Equal(expectedMessage))
			Expect(sink2.Received()[0]).To(Equal(expectedMessage))
		})

		It("only sends to sinks that match the appID", func(done Done) {
			sink1 := &channelSink{appId: "myApp1",
				identifier: "myAppChan1",
				done:       make(chan struct{}),
			}
			sink2 := &channelSink{appId: "myApp2",
				identifier: "myAppChan2",
				done:       make(chan struct{}),
			}

			sinkManager.RegisterSink(sink1)
			sinkManager.RegisterSink(sink2)

			expectedMessageString := "Some Data"
			expectedMessage, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, expectedMessageString, "myApp", "App"), "origin")
			go sinkManager.SendTo("myApp1", expectedMessage)

			Eventually(sink1.Received).Should(HaveLen(1))
			Expect(sink1.Received()[0]).To(Equal(expectedMessage))
			Expect(sink2.Received()).To(BeEmpty())

			close(done)
		})

		It("sends messages to registered firehose sinks", func() {
			sink1 := &channelSink{done: make(chan struct{}), appId: "firehose-a"}
			sinkManager.RegisterFirehoseSink(sink1)

			expectedMessageString := "Some Data"
			expectedMessage, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, expectedMessageString, "myApp", "App"), "origin")
			go sinkManager.SendTo("myApp1", expectedMessage)

			Eventually(sink1.Received).Should(ContainElement(expectedMessage))
		})

		It("sends single message to app sink registered after the message was sent", func() {
			sink1 := &channelSink{appId: "myApp",
				identifier: "myAppChan1",
				done:       make(chan struct{}),
			}

			expectedMessageString := "Some Data"
			expectedMessage, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, expectedMessageString, "myApp", "App"), "origin")
			go sinkManager.SendTo("myApp", expectedMessage)

			sinkManager.RegisterSink(sink1)

			Eventually(sink1.Received).Should(HaveLen(1))
			Expect(sink1.Received()[0]).To(Equal(expectedMessage))
		})

		It("sends single message to firehose sink registered after the message was sent", func() {
			sink1 := &channelSink{done: make(chan struct{}), appId: "firehose-a"}

			expectedMessageString := "Some Data"
			expectedMessage, _ := emitter.Wrap(
				factories.NewLogMessage(events.LogMessage_OUT, expectedMessageString, "myApp", "App"),
				"origin",
			)
			go sinkManager.SendTo("myApp1", expectedMessage)

			sinkManager.RegisterFirehoseSink(sink1)

			Eventually(sink1.Received, 5).Should(ContainElement(expectedMessage))
		})

		Context("When a sync is consuming slowly", func() {
			It("buffers a reasonable number of messages", func() {
				ready := make(chan struct{})
				sink1 := &channelSink{appId: "myApp",
					identifier: "myAppChan1",
					done:       make(chan struct{}),
					ready:      ready,
				}

				sinkManager.RegisterSink(sink1)

				expectedFirstMessageString := "Some Data 1"
				expectedSecondMessageString := "Some Data 2"
				expectedFirstMessage, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, expectedFirstMessageString, "myApp", "App"), "origin")
				expectedSecondMessage, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, expectedSecondMessageString, "myApp", "App"), "origin")

				sinkManager.SendTo("myApp", expectedFirstMessage)
				sinkManager.SendTo("myApp", expectedSecondMessage)

				close(ready)

				Eventually(sink1.Received).Should(HaveLen(2))
				Expect(sink1.Received()[0]).To(Equal(expectedFirstMessage))
				Expect(sink1.Received()[1]).To(Equal(expectedSecondMessage))
			})
		})
	})

	Describe("Start", func() {
		Context("with updates from appstore", func() {
			Context("when an add update is received", func() {
				It("creates a new syslog sink with syslog writer from the newAppServicesChan", func() {
					initialNumSinks := fakeMetricSender.GetValue("messageRouter.numberOfSyslogSinks").Value
					newAppServiceChan <- store.NewServiceInfo("aptastic", "syslog://127.0.1.1:885", "org.space.app.1")

					Eventually(func() float64 { return fakeMetricSender.GetValue("messageRouter.numberOfSyslogSinks").Value }, 2).Should(Equal(initialNumSinks + 1))
				})

				It("creates a new syslog sink with tlsWriter from the newAppServicesChan", func() {
					initialNumSinks := fakeMetricSender.GetValue("messageRouter.numberOfSyslogSinks").Value
					newAppServiceChan <- store.NewServiceInfo("aptastic", "syslog-tls://127.0.1.1:885", "org.space.app.1")

					Eventually(func() float64 { return fakeMetricSender.GetValue("messageRouter.numberOfSyslogSinks").Value }, 2).Should(Equal(initialNumSinks + 1))
				})

				It("creates a new syslog sink with httpsWriter from the newAppServicesChan", func() {
					initialNumSinks := fakeMetricSender.GetValue("messageRouter.numberOfSyslogSinks").Value
					newAppServiceChan <- store.NewServiceInfo("aptastic", "https://127.0.1.1:885", "org.space.app.1")

					Eventually(func() float64 { return fakeMetricSender.GetValue("messageRouter.numberOfSyslogSinks").Value }, 2).Should(Equal(initialNumSinks + 1))
				})

				Context("with an invalid drain Url", func() {
					var errorSink *channelSink

					BeforeEach(func() {
						errorSink = &channelSink{appId: "aptastic",
							identifier: "myAppChan1",
							done:       make(chan struct{}),
						}
						sinkManager.RegisterSink(errorSink)
					})

					It("sends an error message if the drain URL is blacklisted", func() {
						newAppServiceChan <- store.NewServiceInfo("aptastic", "syslog://10.10.10.11:884", "org.space.app.1")
						Eventually(errorSink.Received).Should(HaveLen(1))
						errorMsg := errorSink.Received()[0]
						Expect(string(errorMsg.GetLogMessage().GetMessage())).To(MatchRegexp("Invalid syslog drain URL"))
					})

					It("sends an error message if the drain URL is invalid", func() {
						newAppServiceChan <- store.NewServiceInfo("aptastic", "syslog//invalid", "org.space.app.1")
						Eventually(errorSink.Received).Should(HaveLen(1))
						errorMsg := errorSink.Received()[0]
						Expect(string(errorMsg.GetLogMessage().GetMessage())).To(MatchRegexp("Invalid syslog drain URL"))
					})
				})
			})

			Context("when a delete update is received", func() {
				It("deletes the corresponding syslog sink if it exists", func() {
					initialNumSinks := fakeMetricSender.GetValue("messageRouter.numberOfSyslogSinks").Value
					newAppServiceChan <- store.NewServiceInfo("aptastic", "syslog://127.0.1.1:886", "org.space.app.1")

					Eventually(func() float64 {
						return fakeMetricSender.GetValue("messageRouter.numberOfSyslogSinks").Value
					}, 2).Should(Equal(initialNumSinks + 1))

					deletedAppServiceChan <- store.NewServiceInfo("aptastic", "syslog://127.0.1.1:886", "org.space.app.1")

					Eventually(func() float64 {
						return fakeMetricSender.GetValue("messageRouter.numberOfSyslogSinks").Value
					}, 2).Should(Equal(initialNumSinks))
				})

				It("handles a delete for a nonexistent sink", func() {
					initialNumSinks := fakeMetricSender.GetValue("messageRouter.numberOfSyslogSinks").Value
					deletedAppServiceChan <- store.NewServiceInfo("aptastic", "syslog://127.0.1.1:886", "org.space.app.1")
					Eventually(func() float64 {
						return fakeMetricSender.GetValue("messageRouter.numberOfSyslogSinks").Value
					}, 2).Should(Equal(initialNumSinks))
				})
			})
		})
	})

	Describe("Stop", func() {

		It("stops", func() {
			sinkManager.Stop()
			Eventually(sinkManagerDone).Should(BeClosed())
		})

		It("stops all registered sinks", func(done Done) {
			sink := &channelSink{appId: "myApp1",
				identifier: "myAppChan1",
				done:       make(chan struct{}),
			}
			sinkManager.RegisterSink(sink)
			sinkManager.Stop()
			Expect(sink.RunFinished()).To(BeTrue())

			close(done)
		})

		It("is idempotent", func() {
			sinkManager.Stop()
			Expect(sinkManager.Stop).NotTo(Panic())
		})
	})

	Describe("UnregisterSink", func() {
		Context("with a DumpSink", func() {
			var dumpSink *dump.DumpSink

			BeforeEach(func() {
				health := newSpyHealthRegistrar()
				dumpSink = dump.NewDumpSink("appId", 1, time.Hour, health)
				sinkManager.RegisterSink(dumpSink)
			})

			It("clears the recent logs buffer", func() {
				expectedMessageString := "Some Data"
				expectedMessage, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, expectedMessageString, "myApp", "App"), "origin")
				sinkManager.SendTo("appId", expectedMessage)

				Eventually(func() []*events.Envelope {
					return sinkManager.RecentLogsFor("appId")
				}).Should(HaveLen(1))

				sinkManager.UnregisterSink(dumpSink)

				Eventually(func() []*events.Envelope {
					return sinkManager.RecentLogsFor("appId")
				}).Should(HaveLen(0))
			})
		})

		Context("with a SyslogSink", func() {
			var syslogSink sinks.Sink

			BeforeEach(func() {
				url := &url.URL{Scheme: "syslog", Host: "localhost:9998"}
				writer, _ := syslogwriter.NewSyslogWriter(url, "appId", "loggregator", &net.Dialer{Timeout: 500 * time.Millisecond}, 0)
				syslogSink = syslog.NewSyslogSink("appId", url, 100, writer, func(string, string) {}, "dropsonde-origin")

				sinkManager.RegisterSink(syslogSink)
			})

			It("removes the sink", func() {
				Eventually(func() float64 {
					return fakeMetricSender.GetValue("messageRouter.numberOfSyslogSinks").Value
				}, 2).Should(BeEquivalentTo(1))

				sinkManager.UnregisterSink(syslogSink)

				Eventually(func() float64 {
					return fakeMetricSender.GetValue("messageRouter.numberOfSyslogSinks").Value
				}, 2).Should(BeEquivalentTo(0))
			})
		})

		Context("when called twice", func() {
			var dumpSink *dump.DumpSink

			BeforeEach(func() {
				health := newSpyHealthRegistrar()
				dumpSink = dump.NewDumpSink("appId", 1, time.Hour, health)
				sinkManager.RegisterSink(dumpSink)
			})

			It("decrements the metric only once", func() {
				Eventually(func() float64 {
					return fakeMetricSender.GetValue("messageRouter.numberOfDumpSinks").Value
				}, 2).Should(BeEquivalentTo(1))
				sinkManager.UnregisterSink(dumpSink)
				Eventually(func() float64 {
					return fakeMetricSender.GetValue("messageRouter.numberOfDumpSinks").Value
				}, 2).Should(BeEquivalentTo(0))
				sinkManager.UnregisterSink(dumpSink)
				Eventually(func() float64 {
					return fakeMetricSender.GetValue("messageRouter.numberOfDumpSinks").Value
				}, 2).Should(BeEquivalentTo(0))
			})
		})
	})

	Describe("RegisterFirehoseSink", func() {
		It("runs the sink, updates metrics and returns true for registering a new firehose sink", func() {
			sink := &channelSink{done: make(chan struct{}), appId: "firehose-a"}
			Expect(sinkManager.RegisterFirehoseSink(sink)).To(BeTrue())
			Eventually(sink.RunCalled).Should(BeTrue())

			Eventually(func() float64 {
				return fakeMetricSender.GetValue("messageRouter.numberOfFirehoseSinks").Value
			}, 2).Should(BeEquivalentTo(1))
		})

		It("returns false for a duplicate sink and does not update the sink metrics", func() {
			sink := &channelSink{done: make(chan struct{}), appId: "firehose-a"}

			Expect(sinkManager.RegisterFirehoseSink(sink)).To(BeTrue())

			Expect(sinkManager.RegisterFirehoseSink(sink)).To(BeFalse())
			Eventually(func() float64 {
				return fakeMetricSender.GetValue("messageRouter.numberOfFirehoseSinks").Value
			}, 2).Should(BeEquivalentTo(1))
		})

		It("calls UnregisterFirehoseSink called once Run() returns", func() {
			sink := &channelSink{done: make(chan struct{}), appId: "firehose-a", stop: make(chan struct{})}
			Expect(sinkManager.RegisterFirehoseSink(sink)).To(BeTrue())
			Expect(sinkManager.RegisterFirehoseSink(sink)).To(BeFalse())
			Eventually(sink.RunCalled).Should(BeTrue())

			close(sink.stop)

			Eventually(func() bool { return sinkManager.IsFirehoseRegistered(sink) }).Should(BeFalse())

		})
	})

	Describe("UnregisterFirehoseSink", func() {

		It("stops the sink and updates metrics", func() {
			sink := &channelSink{done: make(chan struct{}), appId: "firehose-a"}

			Expect(sinkManager.RegisterFirehoseSink(sink)).To(BeTrue())

			sinkManager.UnregisterFirehoseSink(sink)
			Eventually(sink.RunFinished).Should(BeTrue())
			Expect(fakeMetricSender.GetValue("messageRouter.numberOfFirehoseSinks").Value).To(Equal(float64(0)))
		})

		It("does not update metrics when a sink is not registered", func() {
			sink := &channelSink{done: make(chan struct{})}

			sinkManager.UnregisterFirehoseSink(sink)
			Expect(fakeMetricSender.GetValue("messageRouter.numberOfFirehoseSinks").Value).To(Equal(float64(0)))
		})
	})

	Describe("IsFirehoseSinkRegistered", func() {
		It("returns true if sink is registered", func() {
			sink := &channelSink{done: make(chan struct{}), appId: "firehose-a"}

			Expect(sinkManager.RegisterFirehoseSink(sink)).To(BeTrue())
			Expect(sinkManager.IsFirehoseRegistered(sink)).To(BeTrue())
		})

		It("returns false if sink is not registered", func() {
			sink := &channelSink{done: make(chan struct{}), appId: "firehose-a"}

			Expect(sinkManager.IsFirehoseRegistered(sink)).To(BeFalse())
		})

		It("returns false if sink is not registered", func() {
			sink := &channelSink{done: make(chan struct{}), appId: "firehose-a"}
			Expect(sinkManager.RegisterFirehoseSink(sink)).To(BeTrue())
			sinkManager.UnregisterFirehoseSink(sink)
			Expect(sinkManager.IsFirehoseRegistered(sink)).To(BeFalse())
		})
	})

	Describe("Latest Container Metrics", func() {
		var sink *channelSink
		BeforeEach(func() {
			sink = &channelSink{
				appId:      "myApp",
				identifier: "myAppChan1",
				done:       make(chan struct{}),
			}
			sinkManager.RegisterSink(sink)
		})

		It("sends latest container metrics for a given app", func() {
			env := &events.Envelope{
				EventType: events.Envelope_ContainerMetric.Enum(),
				Timestamp: proto.Int64(time.Now().UnixNano()),
				ContainerMetric: &events.ContainerMetric{
					ApplicationId: proto.String("myApp"),
					InstanceIndex: proto.Int32(1),
					CpuPercentage: proto.Float64(73),
					MemoryBytes:   proto.Uint64(2),
					DiskBytes:     proto.Uint64(3),
				},
			}

			sinkManager.SendTo("myApp", env)

			Eventually(func() []*events.Envelope { return sinkManager.LatestContainerMetrics("myApp") }).Should(ConsistOf(env))
		})
	})

	Describe("SendSyslogErrorToLoggregator", func() {
		It("listens and broadcasts error messages", func() {
			sink := &channelSink{
				appId:      "myApp",
				identifier: "myAppChan1",
				done:       make(chan struct{}),
			}
			sinkManager.RegisterSink(sink)
			sinkManager.SendSyslogErrorToLoggregator("error msg", "myApp")

			Eventually(sink.Received).Should(HaveLen(1))

			errorMsg := sink.Received()[0]
			Expect(string(errorMsg.GetLogMessage().GetMessage())).To(Equal("error msg"))
		})
	})
})

type channelSink struct {
	sync.RWMutex
	done              chan struct{}
	appId, identifier string
	received          []*events.Envelope
	runCalled         bool
	ready             chan struct{}
	stop              chan struct{}
}

func (c *channelSink) AppID() string { return c.appId }
func (c *channelSink) Run(msgChan <-chan *events.Envelope) {
	if c.ready != nil {
		<-c.ready
	}
	c.Lock()
	c.runCalled = true
	c.Unlock()

	defer close(c.done)
	for {
		select {
		case msg, ok := <-msgChan:
			if !ok {
				return
			}
			c.Lock()
			c.received = append(c.received, msg)
			c.Unlock()
		case <-c.stop:
			return
		}
	}
}

func (c *channelSink) RunFinished() bool {
	<-c.done
	return true
}

func (c *channelSink) RunCalled() bool {
	c.RLock()
	defer c.RUnlock()
	return c.runCalled
}

func (c *channelSink) Received() []*events.Envelope {
	c.RLock()
	defer c.RUnlock()
	data := make([]*events.Envelope, len(c.received))
	copy(data, c.received)
	return data
}

func (c *channelSink) Identifier() string        { return c.identifier }
func (c *channelSink) ShouldReceiveErrors() bool { return true }

func (c *channelSink) GetInstrumentationMetric() sinks.Metric {
	return sinks.Metric{Name: "numberOfMessagesLost", Value: 25}
}

type SpyHealthRegistrar struct {
	mu     sync.Mutex
	values map[string]float64
}

func newSpyHealthRegistrar() *SpyHealthRegistrar {
	return &SpyHealthRegistrar{
		values: make(map[string]float64),
	}
}

func (s *SpyHealthRegistrar) Inc(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[name]++
}

func (s *SpyHealthRegistrar) Dec(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[name]--
}

func (s *SpyHealthRegistrar) Get(name string) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.values[name]
}
