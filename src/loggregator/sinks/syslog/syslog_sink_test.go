package syslog_test

import (
	"errors"
	"fmt"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sync"
	"time"
	"loggregator/sinks/syslog"
)

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
	return w.connected
}

func (w *SyslogWriterRecorder) SetConnected(newValue bool) {
	w.connected = newValue
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
	var syslogSink *syslog.SyslogSink
	var sysLogger *SyslogWriterRecorder
	var sysLoggerDoneChan chan bool
	var errorChannel chan *logmessage.Message
	var mutex sync.Mutex
	var inputChan chan *logmessage.Message

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
		inputChan = make(chan *logmessage.Message)
		syslogSink = syslog.NewSyslogSink("appId", "syslog://using-fake", loggertesthelper.Logger(), sysLogger, errorChannel).(*syslog.SyslogSink)
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
				syslogSink.Run(inputChan)
				closeSysLoggerDoneChan()
			}()
		})

		It("should still accept messages without blocking", func(done Done) {
			logMessage := NewMessage("test message", "appId")

			for i := 0; i < 100; i++ {
				inputChan <- logMessage
			}
			close(done)
		})

		Context("when remote syslog server comes up", func() {
			BeforeEach(func() {
				for i := 0; i < 5; i++ {
					inputChan <- NewMessage(fmt.Sprintf("test message: %d\n", i), "appId")
				}
				close(inputChan)

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
				syslogSink.Run(inputChan)
				closeSysLoggerDoneChan()
			}()
		})

		It("Uses the timestamp of the logmessage when sending", func(done Done) {
			message := NewMessage("test message", "appId")
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

			It("should report error messages when it's connected", func(done Done) {
				logMessage := NewMessage("test message", "appId")
				inputChan <- logMessage
				errorLog := <-errorChannel
				errorMsg := string(errorLog.GetLogMessage().GetMessage())
				Expect(errorMsg).To(MatchRegexp(`Syslog Sink syslog://using-fake: Error when dialing out. Backing off for (\d\.\d+ms|\d+us). Err: Error connecting.`))
				Expect(errorLog.GetLogMessage().GetSourceName()).To(Equal("LGR"))

				close(done)
			})

			It("should not report error messages when it's disconnected", func(done Done) {
				syslogSink.Disconnect()

				logMessage := NewMessage("test message", "appId")
				inputChan <- logMessage
				Expect(errorChannel).To(BeEmpty())
				close(done)
			})

			Context("when the buffer overflows", func() {

				BeforeEach(func() {
					for i := 0; i < 105; i++ {
						msg := fmt.Sprintf("message no %v", i)
						logMessage := NewMessage(msg, "appId")

						inputChan <- logMessage
					}
					close(inputChan)
				})

				Context("when the remote syslog server comes back up again", func() {
					BeforeEach(func() {
						sysLogger.SetDown(false)
						<-sysLoggerDoneChan
					})

					It("should resume sending messages", func(done Done) {
						data := sysLogger.ReceivedMessages()
						Expect(data).To(HaveLen(6))
						for i := 0; i < 5; i++ {
							msg := fmt.Sprintf("out: message no %v", i+100)
							Expect(data[i+1]).To(MatchRegexp(msg))
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
