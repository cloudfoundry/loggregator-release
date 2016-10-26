package syslog_test

import (
	"doppler/sinks/syslog"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const bufferSize = 100

var _ = Describe("SyslogSink", func() {
	var (
		syslogSink            *syslog.SyslogSink
		syslogSinkMetrics     *syslog.SyslogSink
		sysLogger             *SyslogWriterRecorder
		syslogSinkRunFinished chan bool
		errorChannel          chan *events.Envelope
		errorHandler          func(string, string)
		inputChan             chan *events.Envelope
		dialer                *net.Dialer
		logger                *gosteno.Logger
		drainURL              string
	)

	BeforeEach(func() {
		syslogSinkRunFinished = make(chan bool)
		sysLogger = NewSyslogWriterRecorder()
		errorChannel = make(chan *events.Envelope, 10)
		inputChan = make(chan *events.Envelope)
		dialer = &net.Dialer{}
		logger = loggertesthelper.Logger()
		drainURL = "syslog://using-fake"

		errorHandler = func(errorMsg, appId string) {
			logMessage := factories.NewLogMessage(events.LogMessage_ERR, errorMsg, appId, "LGR")
			envelope, _ := emitter.Wrap(logMessage, "dropsonde-origin")

			select {
			case errorChannel <- envelope:
			default:
			}
		}
	})

	JustBeforeEach(func() {
		drainURL, err := url.Parse(drainURL)
		Expect(err).ToNot(HaveOccurred())
		syslogSink = syslog.NewSyslogSink("appId", drainURL, logger, bufferSize, sysLogger, errorHandler, "dropsonde-origin", false)
		syslogSinkMetrics = syslog.NewSyslogSink("appId", drainURL, logger, bufferSize, sysLogger, errorHandler, "dropsonde-origin", true)
	})

	Describe("Identifier", func() {
		Context("with an empty drain URL", func() {
			BeforeEach(func() {
				drainURL = ""
			})

			It("returns an empty string", func() {
				Expect(syslogSink.Identifier()).To(BeEmpty())
			})
		})
	})

	Context("hides sensitive info", func() {

		Context("https", func() {
			BeforeEach(func() {
				drainURL = "https://testuser:testpass@syslog-host/some/location?user=testuser&password=testpass"
			})

			It("displays only the drain URL host and path", func() {
				Expect(loggertesthelper.TestLoggerSink.LogContents()).ToNot(ContainSubstring("testpass"))
				Expect(loggertesthelper.TestLoggerSink.LogContents()).To(ContainSubstring("Syslog Sink https://syslog-host/some/location: Created for appId [appId]"))
			})

			It("returns an identifier without a user or password", func() {
				identifier := syslogSink.Identifier()
				Expect(identifier).NotTo(ContainSubstring("testuser"))
				Expect(identifier).NotTo(ContainSubstring("testpass"))
			})
		})

		Context("syslog", func() {
			BeforeEach(func() {
				drainURL = "syslog://syslog-host?user=testuser&password=testpass"
			})

			It("displays only the drain URL host and path", func() {
				Expect(loggertesthelper.TestLoggerSink.LogContents()).ToNot(ContainSubstring("testpass"))
				Expect(loggertesthelper.TestLoggerSink.LogContents()).To(ContainSubstring("Syslog Sink syslog://syslog-host: Created for appId [appId]"))
			})

			It("returns an identifier without a user or password", func() {
				identifier := syslogSink.Identifier()
				Expect(identifier).NotTo(ContainSubstring("testuser"))
				Expect(identifier).NotTo(ContainSubstring("testpass"))
			})
		})

		Context("when running a syslog sink", func() {
			BeforeEach(func() {
				drainURL = "https://testuser:testpass@syslog-host/some/location?user=testuser&password=testpass"
			})

			JustBeforeEach(func() {
				go func() {
					syslogSink.Run(inputChan)
					close(syslogSinkRunFinished)
				}()
			})

			AfterEach(func() {
				syslogSink.Disconnect()
				Eventually(syslogSinkRunFinished).Should(BeClosed())
			})

			It("displays only the drainURL host and path", func() {
				logMessage := factories.NewLogMessage(events.LogMessage_OUT, "test message", "appId", "App")
				logMessage.SourceInstance = proto.String("123")

				envelope, _ := emitter.Wrap(logMessage, "origin")
				inputChan <- envelope

				Expect(loggertesthelper.TestLoggerSink.LogContents()).ToNot(ContainSubstring("testpass"))
				Expect(loggertesthelper.TestLoggerSink.LogContents()).To(ContainSubstring("Syslog Sink https://syslog-host/some/location"))
			})

		})
	})

	Context("when remote syslog server is down", func() {
		JustBeforeEach(func() {
			sysLogger.SetDown(true)
			go func() {
				syslogSink.Run(inputChan)
				close(syslogSinkRunFinished)
			}()
		})

		AfterEach(func() {
			syslogSink.Disconnect()
			Eventually(syslogSinkRunFinished).Should(BeClosed())
		})

		It("still accepts messages without blocking", func() {
			logMessage, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "test message", "appId", "App"), "origin")

			for i := 0; i < bufferSize; i++ {
				Eventually(inputChan).Should(BeSent(logMessage))
			}
		})

		Context("when remote syslog server comes up", func() {
			var numMessages int

			BeforeEach(func() {
				numMessages = 5
			})

			JustBeforeEach(func() {
				for i := 0; i < numMessages; i++ {
					message, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, fmt.Sprintf("test message: %d\n", i), "appId", "App"), "origin")
					inputChan <- message
				}
				close(inputChan)

				sysLogger.SetDown(false)
			})

			It("sends all the messages in the buffer", func() {
				Eventually(syslogSinkRunFinished).Should(BeClosed())
				data := sysLogger.ReceivedMessages()
				Expect(data).To(HaveLen(5))
			})
		})
	})

	Context("when remote syslog server is up", func() {
		JustBeforeEach(func() {
			go func(*syslog.SyslogSink, chan bool) {
				syslogSink.Run(inputChan)
				close(syslogSinkRunFinished)
			}(syslogSink, syslogSinkRunFinished)
		})

		AfterEach(func() {
			syslogSink.Disconnect()
			Eventually(syslogSinkRunFinished).Should(BeClosed())
		})

		It("sends input messages to the syslog writer", func(done Done) {
			logMessage := factories.NewLogMessage(events.LogMessage_OUT, "test message", "appId", "App")
			logMessage.SourceInstance = proto.String("123")
			envelope, _ := emitter.Wrap(logMessage, "origin")

			inputChan <- envelope
			data := <-sysLogger.receivedChannel

			expectedSyslogMessage := fmt.Sprintf(`<14>1 test message ts: \d+ src: App srcId: 123`)
			Expect(string(data)).To(MatchRegexp(expectedSyslogMessage))
			close(done)
		})

		It("does not send container metrics to the syslog writer", func(done Done) {
			containerMetric := factories.NewContainerMetric("appId", 0, 2.6, 123456, 500)
			envelope, _ := emitter.Wrap(containerMetric, "origin")

			inputChan <- envelope

			Consistently(sysLogger.receivedChannel).ShouldNot(Receive())
			close(done)
		})

		It("does not send not-allowed messages to the syslog writer", func(done Done) {
			nonLogMessage := factories.NewValueMetric("value-name", 2.0, "value-unit")
			envelope, _ := emitter.Wrap(nonLogMessage, "origin")

			inputChan <- envelope

			Consistently(sysLogger.receivedChannel).ShouldNot(Receive())
			close(done)
		})

		It("stops sending messages when the disconnect comes in", func(done Done) {
			logMessage, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "test message", "appId", "App"), "origin")
			inputChan <- logMessage

			_, ok := <-sysLogger.receivedChannel
			Expect(ok).To(BeTrue())

			syslogSink.Disconnect()

			Eventually(syslogSinkRunFinished).Should(BeClosed())

			close(done)
		})

		It("uses the timestamp of the logmessage when sending", func(done Done) {
			message, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "test message", "appId", "App"), "origin")
			expectedTimeString := fmt.Sprintf("ts: %d", message.GetLogMessage().GetTimestamp())

			time.Sleep(100 * time.Millisecond) //wait a bit to allow timestamps to differ

			inputChan <- message
			data := <-sysLogger.receivedChannel

			Expect(string(data)).To(MatchRegexp(expectedTimeString))
			close(done)
		})

		Context("when remote syslog server goes down", func() {
			BeforeEach(func() {
				sysLogger.SetDown(true)
			})

			It("reports error messages when it's connected", func() {
				logMessage, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "test message", "appId", "App"), "origin")
				inputChan <- logMessage
				errorLog := <-errorChannel
				errorMsg := string(errorLog.GetLogMessage().GetMessage())
				Expect(errorMsg).To(MatchRegexp(`Syslog Sink syslog://using-fake: Error when dialing out. Backing off for (\d+(\.\d+)?(m|u|µ)s). Err: Error connecting.`))
				Expect(errorLog.GetLogMessage().GetSourceType()).To(Equal("LGR"))
			})

			It("stops sending messages when the disconnect comes in", func() {
				logMessage, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "test message", "appId", "App"), "origin")
				inputChan <- logMessage

				Eventually(errorChannel).ShouldNot(BeEmpty())
				syslogSink.Disconnect()
				Eventually(syslogSinkRunFinished).Should(BeClosed())
				numErrors := len(errorChannel)

				logMessage, _ = emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "test message 2", "appId", "App"), "origin")

				Expect(inputChan).ShouldNot(BeSent(logMessage))
				close(inputChan)

				Expect(errorChannel).To(HaveLen(numErrors))
			})

			Context("when the buffer overflows", func() {
				JustBeforeEach(func() {
					for i := 0; i < bufferSize+5; i++ {
						msg := fmt.Sprintf("message no %v", i)
						logMessage, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, msg, "appId", "App"), "origin")

						inputChan <- logMessage
					}
					close(inputChan)
				})

				Context("when the remote syslog server comes back up again", func() {
					JustBeforeEach(func() {
						sysLogger.SetDown(false)
						Eventually(syslogSinkRunFinished).Should(BeClosed())
					})

					It("resumes sending messages", func() {
						data := sysLogger.ReceivedMessages()
						Expect(data).To(HaveLen(6))
						Expect(data[0]).To(ContainSubstring(fmt.Sprintf("<14>1 message no 0")))

						for i := 2; i < 6; i++ {
							msg := fmt.Sprintf("<14>1 message no %v", i-1+bufferSize)
							Expect(data[i]).To(ContainSubstring(msg))
						}
					})

					It("sends a message about the buffer overflow", func() {
						data := sysLogger.ReceivedMessages()
						Expect(len(data)).To(BeNumerically(">", 2))
						Expect(data[1]).To(MatchRegexp("<11>1 Log message output is too high. 100 messages dropped"))
					})
				})
			})
		})
	})

	Context("when remote syslog server is up and container metrics are allowed", func() {
		JustBeforeEach(func() {
			go func(*syslog.SyslogSink, chan bool) {
				syslogSinkMetrics.Run(inputChan)
				close(syslogSinkRunFinished)
			}(syslogSinkMetrics, syslogSinkRunFinished)
		})

		AfterEach(func() {
			syslogSinkMetrics.Disconnect()
			Eventually(syslogSinkRunFinished).Should(BeClosed())
		})

		It("sends container metrics to the syslog writer", func(done Done) {
                        containerMetric := factories.NewContainerMetric("appId", 0, 2.6, 123456, 500)
                        envelope, _ := emitter.Wrap(containerMetric, "origin")

                        inputChan <- envelope
                        data := <-sysLogger.receivedChannel

                        Expect(string(data)).To(MatchRegexp("diskBytes:500"))
                        Expect(string(data)).To(MatchRegexp("memoryBytes:123456"))
                        Expect(string(data)).To(MatchRegexp("cpuPercentage:2.6"))
                        close(done)
                })

	})

	Describe("Disconnect", func() {
		It("is idempotent", func() {
			syslogSink.Disconnect()
			Expect(syslogSink.Disconnect).NotTo(Panic())
		})
	})

	Describe("Exponentially backs off", func() {
		var timestamps []time.Time
		var errors int64

		BeforeEach(func() {
			errors = 0
			timestamps = []time.Time{}
			errorHandler = func(errorMsg, appId string) {
				timestamps = append(timestamps, time.Now())
				atomic.AddInt64(&errors, 1)
			}
		})

		JustBeforeEach(func() {
			sysLogger.SetDown(true)
			go func() {
				syslogSink.Run(inputChan)
				close(syslogSinkRunFinished)
			}()
		})

		AfterEach(func() {
			syslogSink.Disconnect()
			Eventually(syslogSinkRunFinished).Should(BeClosed())
		})

		It("backsoff when retrying", func() {
			logMessage, _ := emitter.Wrap(factories.NewLogMessage(events.LogMessage_OUT, "a message", "appId", "App"), "origin")
			inputChan <- logMessage

			close(inputChan)

			Eventually(func() int64 {
				return atomic.LoadInt64(&errors)
			}).Should(BeNumerically(">", 5))

			// We ignore the difference in timestamps for the 0th iteration because our exponential backoff
			// strategy starts of with a difference of 1 ms
			var diff, prevDiff int64
			for i := 1; i < 5; i++ {
				delta := timestamps[i+1].Sub(timestamps[i])
				Expect(delta).To(BeNumerically(">", 2*prevDiff))
				prevDiff = diff
			}
		})
	})
})

type SyslogWriterRecorder struct {
	receivedChannel  chan string
	receivedMessages []string
	down             bool
	connected        bool
	sync.Mutex
}

func NewSyslogWriterRecorder() *SyslogWriterRecorder {
	return &SyslogWriterRecorder{
		receivedChannel: make(chan string, 20),
		connected:       false,
	}
}

func (r *SyslogWriterRecorder) Connect() error {
	r.Lock()
	defer r.Unlock()
	if r.down {
		r.connected = false
		return errors.New("Error connecting.")
	} else {
		r.connected = true
		return nil
	}
}

func (r *SyslogWriterRecorder) Write(p int, b []byte, source, sourceId string, timestamp int64) (int, error) {
	r.Lock()
	defer r.Unlock()

	if r.down {
		return 0, errors.New("Error writing to stdout.")
	}

	messageString := fmt.Sprintf("<%d>1 %s ts: %d src: %s srcId: %s", p, string(b), timestamp, source, sourceId)
	r.receivedMessages = append(r.receivedMessages, messageString)
	r.receivedChannel <- messageString
	return len(b), nil
}

func (r *SyslogWriterRecorder) SetDown(newState bool) {
	r.Lock()
	defer r.Unlock()

	r.down = newState
}

func (r *SyslogWriterRecorder) IsConnected() bool {
	r.Lock()
	defer r.Unlock()
	return r.connected
}

func (r *SyslogWriterRecorder) SetConnected(newValue bool) {
	r.Lock()
	defer r.Unlock()
	r.connected = newValue
}

func (r *SyslogWriterRecorder) Close() error {
	return nil
}

func (r *SyslogWriterRecorder) ReceivedMessages() []string {
	r.Lock()
	defer r.Unlock()

	return r.receivedMessages
}
