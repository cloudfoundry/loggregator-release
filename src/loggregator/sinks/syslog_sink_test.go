package sinks_test

import (
	"errors"
	"fmt"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"loggregator/sinks"
	"sync"
	"time"
)

type SyslogWriterRecorder struct {
	receivedChannel  chan string
	receivedMessages []string
	down             bool
	disconnected     bool
	sync.Mutex
}

func NewSyslogWriterRecorder() *SyslogWriterRecorder {
	return &SyslogWriterRecorder{
		receivedChannel: make(chan string, 20),
	}
}

func (r *SyslogWriterRecorder) Connect() error {
	r.Lock()
	defer r.Unlock()

	if r.down {
		r.disconnected = true
		return errors.New("Error connecting.")
	} else {
		r.disconnected = false
		return nil
	}
}

func (r *SyslogWriterRecorder) WriteStdout(b []byte, source, sourceId string, timestamp int64) (int, error) {
	r.Lock()
	defer r.Unlock()

	if r.down {
		return 0, errors.New("Error writing to stdout.")
	}

	messageString := fmt.Sprintf("out: %s ts: %d src: %s srcId: %s", string(b), timestamp, source, sourceId)
	r.receivedMessages = append(r.receivedMessages, messageString)
	r.receivedChannel <- messageString
	return len(b), nil
}

func (r *SyslogWriterRecorder) WriteStderr(b []byte, source, sourceId string, timestamp int64) (int, error) {
	r.Lock()
	defer r.Unlock()

	if r.down {
		return 0, errors.New("Error writing to stderr.")
	}

	messageString := fmt.Sprintf("err: %s ts: %d src: %s srcId: %s", string(b), timestamp, source, sourceId)
	r.receivedMessages = append(r.receivedMessages, messageString)
	r.receivedChannel <- messageString
	return len(b), nil
}

func (r *SyslogWriterRecorder) SetDown(newState bool) {
	r.Lock()
	defer r.Unlock()

	r.down = newState
}

func (w *SyslogWriterRecorder) IsConnected() bool {
	return !w.disconnected
}

func (w *SyslogWriterRecorder) SetConnected(newValue bool) {
	w.disconnected = !newValue
}

func (r *SyslogWriterRecorder) Close() error {
	return nil
}

func (r *SyslogWriterRecorder) ReceivedMessages() []string {
	r.Lock()
	defer r.Unlock()

	return r.receivedMessages
}

var _ = Describe("SyslogSink", func() {
	var syslogSink sinks.Sink
	var sysLogger *SyslogWriterRecorder
	var sysLoggerDoneChan chan bool
	var errorChannel chan *logmessage.Message
	var mutex sync.Mutex

	closeSysLoggerDoneChan := func() {
		mutex.Lock()
		defer mutex.Unlock()
		close(sysLoggerDoneChan)
	}

	newSysLoggerDoneChan := func() {
		mutex.Lock()
		defer mutex.Unlock()
		sysLoggerDoneChan = make(chan bool)
	}

	BeforeEach(func() {
		newSysLoggerDoneChan()
		sysLogger = NewSyslogWriterRecorder()
		errorChannel = make(chan *logmessage.Message, 10)
		syslogSink = sinks.NewSyslogSink("appId", "syslog://localhost:24632", loggertesthelper.Logger(), sysLogger, errorChannel)
	})

	AfterEach(func() {
		select {
		case <-sysLoggerDoneChan:
		default:
			closeSysLoggerDoneChan()
		}
	})

	Context("when remote syslog server is down", func() {
		BeforeEach(func() {
			sysLogger.SetDown(true)
			go func() {
				syslogSink.Run()
				closeSysLoggerDoneChan()
			}()
		})

		It("should still accept messages without blocking", func(done Done) {
			logMessage := NewMessage("test message", "appId")

			for i := 0; i < 100; i++ {
				syslogSink.Channel() <- logMessage
			}
			close(done)
		})

		Context("when remote syslog server comes up", func() {
			BeforeEach(func() {
				logMessage := NewMessage("test message", "appId")

				for i := 0; i < 5; i++ {
					syslogSink.Channel() <- logMessage
				}
				close(syslogSink.Channel())
				sysLogger.SetDown(false)
			})

			It("should send all the messages in the buffer", func(done Done) {
				<-sysLoggerDoneChan
				data := sysLogger.ReceivedMessages()
				Expect(data).To(HaveLen(5))
				close(done)
			})
		})
	})

	Context("when remote syslog server is up", func() {

		BeforeEach(func() {
			go func() {
				syslogSink.Run()
				closeSysLoggerDoneChan()
			}()
		})

		It("Uses the timestamp of the logmessage when sending", func(done Done) {
			message := NewMessage("test message", "appId")
			expectedTimeString := fmt.Sprintf("ts: %d", message.GetLogMessage().GetTimestamp())

			time.Sleep(100 * time.Millisecond) //wait a bit to allow timestamps to differ

			syslogSink.Channel() <- message
			data := <-sysLogger.receivedChannel

			Expect(string(data)).To(MatchRegexp(expectedTimeString))
			close(done)
		})

		Context("when remote syslog server goes down", func() {
			BeforeEach(func() {
				sysLogger.SetDown(true)
			})

			It("should report error messages", func(done Done) {
				logMessage := NewMessage("test message", "appId")
				syslogSink.Channel() <- logMessage
				errorLog := <-errorChannel
				errorMsg := string(errorLog.GetLogMessage().GetMessage())
				Expect(errorMsg).To(MatchRegexp(`Syslog Sink syslog://localhost:24632: Error when dialing out. Backing off for \d\.\d+ms. Err: Error connecting.`))
				Expect(errorLog.GetLogMessage().GetSourceName()).To(Equal("LGR"))
				close(done)
			})

			Context("when the buffer overflows", func() {

				BeforeEach(func() {
					for i := 0; i < 105; i++ {
						msg := fmt.Sprintf("message no %v", i)
						logMessage := NewMessage(msg, "appId")

						syslogSink.Channel() <- logMessage
					}
					close(syslogSink.Channel())
				})

				Context("when the remote syslog server comes back up again", func() {
					BeforeEach(func() {
						sysLogger.SetDown(false)
						<-sysLoggerDoneChan
					})

					It("should resume sending messages", func(done Done) {
						data := sysLogger.ReceivedMessages()
						Expect(data).To(HaveLen(5))
						for i := 1; i < 5; i++ {
							msg := fmt.Sprintf("out: message no %v", i+100)
							Expect(data[i]).To(MatchRegexp(msg))
						}
						close(done)
					})

					It("should send a message about the buffer overflow", func(done Done) {
						data := sysLogger.ReceivedMessages()
						Expect(len(data)).To(BeNumerically(">", 1))
						Expect(data[0]).To(MatchRegexp("err: Log message output too high. We've dropped 100 messages"))
						close(done)
					})
				})
			})

		})
	})
})
