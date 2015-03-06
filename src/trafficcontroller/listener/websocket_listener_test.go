package listener_test

import (
	"trafficcontroller/listener"

	"fmt"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"
	"trafficcontroller/marshaller"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WebsocketListener", func() {
	var ts *httptest.Server
	var messageChan, outputChan chan []byte
	var stopChan chan struct{}
	var l listener.Listener
	var fh *fakeHandler

	BeforeEach(func() {
		messageChan = make(chan []byte)
		outputChan = make(chan []byte, 10)
		stopChan = make(chan struct{})
		fh = &fakeHandler{messages: messageChan}
		ts = httptest.NewUnstartedServer(fh)
		converter := func(d []byte) ([]byte, error) { return d, nil }
		l = listener.NewWebsocket(marshaller.LoggregatorLogMessage, converter, 500*time.Millisecond, loggertesthelper.Logger())
	})

	AfterEach(func() {
		select {
		case <-messageChan:
			// already closed
		default:
			close(messageChan)
		}
		ts.Close()
	})

	Context("when the server is not running", func() {
		It("should error when connecting", func(done Done) {
			err := l.Start("ws://localhost:1234", "myApp", outputChan, stopChan)
			Expect(err).To(HaveOccurred())
			close(done)
		}, 2)
	})

	Context("when the server is running", func() {
		BeforeEach(func() {
			ts.Start()
			Eventually(func() bool {
				resp, _ := http.Head(fmt.Sprintf("http://%s", ts.Listener.Addr()))
				return resp != nil && resp.StatusCode == http.StatusOK
			}).Should(BeTrue())
		})

		It("should connect to a websocket", func(done Done) {
			doneWaiting := make(chan struct{})
			go func() {
				defer GinkgoRecover()
				err := l.Start(fmt.Sprintf("ws://%s", ts.Listener.Addr()), "myApp", outputChan, stopChan)
				Expect(err).NotTo(HaveOccurred())
				close(doneWaiting)
			}()
			close(stopChan)
			Eventually(doneWaiting).Should(BeClosed())
			close(done)
		})

		It("should output messages recieved from the server", func(done Done) {
			go l.Start(fmt.Sprintf("ws://%s", ts.Listener.Addr()), "myApp", outputChan, stopChan)

			message := []byte("hello world")
			messageChan <- message

			var receivedMessage []byte
			Eventually(outputChan).Should(Receive(&receivedMessage))
			Expect(receivedMessage).To(Equal(message))

			close(done)
		})

		It("should not send errors when client requests close without issue", func() {
			doneWaiting := make(chan struct{})
			go func() {
				l.Start(fmt.Sprintf("ws://%s", ts.Listener.Addr()), "myApp", outputChan, stopChan)
				close(doneWaiting)
			}()
			close(stopChan)
			Eventually(doneWaiting).Should(BeClosed())
			Consistently(outputChan).Should(BeEmpty())
		})

		It("should not send errors when server closes connection without issue", func() {
			doneWaiting := make(chan struct{})
			go func() {
				l.Start(fmt.Sprintf("ws://%s", ts.Listener.Addr()), "myApp", outputChan, stopChan)
				close(doneWaiting)
			}()
			close(messageChan)
			Eventually(doneWaiting).Should(BeClosed())
			Consistently(outputChan).Should(BeEmpty())
		})

		It("should stop all goroutines when done", func() {
			doneWaiting := make(chan struct{})
			go func() {
				l.Start(fmt.Sprintf("ws://%s", ts.Listener.Addr()), "myApp", outputChan, stopChan)
				close(doneWaiting)
			}()
			close(stopChan)
			Consistently(outputChan).ShouldNot(BeClosed())
			Eventually(doneWaiting).Should(BeClosed())
		})

		It("should stop all goroutines when server returns an error", func(done Done) {
			doneWaiting := make(chan struct{})
			go func() {
				l.Start(fmt.Sprintf("ws://%s", ts.Listener.Addr()), "myApp", outputChan, stopChan)
				close(doneWaiting)
			}()

			// Ensure listener is up by sending message through
			message := []byte("hello world")
			messageChan <- message
			outMessage := <-outputChan
			Expect(outMessage).To(Equal(message))

			// Take server down to cause listener to go down
			close(messageChan)
			Consistently(outputChan).ShouldNot(BeClosed())
			Consistently(stopChan).ShouldNot(BeClosed())
			Eventually(doneWaiting).Should(BeClosed())
			close(done)
		})
	})

	Context("when the server is slow", func() {
		BeforeEach(func() {
			ts.Start()
			Eventually(func() bool {
				resp, _ := http.Head(fmt.Sprintf("http://%s", ts.Listener.Addr()))
				return resp != nil && resp.StatusCode == http.StatusOK
			}).Should(BeTrue())
		})

		Context("with a timeout set", func() {
			It("does not wait forever", func(done Done) {
				err := l.Start(fmt.Sprintf("ws://%s", ts.Listener.Addr()), "myApp", outputChan, stopChan)
				Expect(err).To(HaveOccurred())

				close(done)
			})

			It("sends an error message to the channel", func(done Done) {
				l.Start(fmt.Sprintf("ws://%s", ts.Listener.Addr()), "myApp", outputChan, stopChan)
				var msgData []byte
				Eventually(outputChan).Should(Receive(&msgData))

				msg, _ := logmessage.ParseMessage(msgData)
				Expect(msg.GetLogMessage().GetSourceName()).To(Equal("LGR"))
				Expect(string(msg.GetLogMessage().GetMessage())).To(Equal("WebsocketListener.Start: Timed out listening to a doppler server after 500ms"))
				close(done)
			})
		})

		Context("without a timeout", func() {
			It("waits for messages to come in", func() {
				converter := func(d []byte) ([]byte, error) { return d, nil }
				l = listener.NewWebsocket(marshaller.LoggregatorLogMessage, converter, 0, loggertesthelper.Logger())

				go l.Start(fmt.Sprintf("ws://%s", ts.Listener.Addr()), "myApp", outputChan, stopChan)

				go func() {
					time.Sleep(750 * time.Millisecond)
					message := []byte("hello world")
					messageChan <- message
				}()

				var msgData []byte
				Eventually(outputChan).Should(Receive(&msgData))
				Expect(msgData).To(BeEquivalentTo("hello world"))
			})

			It("responds to stopChan closure in a reasonable time", func(done Done) {
				converter := func(d []byte) ([]byte, error) { return d, nil }
				l = listener.NewWebsocket(marshaller.LoggregatorLogMessage, converter, 0, loggertesthelper.Logger())

				go func() {
					l.Start(fmt.Sprintf("ws://%s", ts.Listener.Addr()), "myApp", outputChan, stopChan)
					close(done)
				}()
				time.Sleep(10 * time.Millisecond)
				close(stopChan)
			})
		})
	})

	Context("when the server has errors", func() {
		BeforeEach(func() {
			ts.Start()
			go l.Start(fmt.Sprintf("ws://%s", ts.Listener.Addr()), "myApp", outputChan, stopChan)
			fh.CloseAbruptly()
		})

		It("should send an error message to the channel", func(done Done) {
			msgData := <-outputChan
			msg, _ := logmessage.ParseMessage(msgData)
			Expect(msg.GetLogMessage().GetSourceName()).To(Equal("LGR"))
			Expect(string(msg.GetLogMessage().GetMessage())).To(Equal("WebsocketListener.Start: Error connecting to a doppler server"))
			close(done)
		})
	})
})

type fakeHandler struct {
	messages   chan []byte
	lastWSConn *websocket.Conn
	sync.Mutex
}

func (f *fakeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == "HEAD" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	ws, err := websocket.Upgrade(w, r, nil, 0, 0)
	if _, ok := err.(websocket.HandshakeError); ok {
		http.Error(w, "Not a websocket handshake", 400)
		return
	} else if err != nil {
		log.Println(err)
		return
	}
	defer ws.Close()
	defer ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Time{})
	f.Lock()
	f.lastWSConn = ws
	f.Unlock()

	go func() {
		ws.ReadMessage()

		select {
		case <-f.messages:
		default:
			close(f.messages)
		}
	}()

	for msg := range f.messages {
		if err := ws.WriteMessage(websocket.BinaryMessage, msg); err != nil {
			return
		}
	}
}

func (f *fakeHandler) Close() {
	close(f.messages)
}

func (f *fakeHandler) CloseAbruptly() {
	Eventually(f.lastConn).ShouldNot(BeNil())
	f.lastConn().Close()
}

func (f *fakeHandler) lastConn() *websocket.Conn {
	f.Lock()
	defer f.Unlock()
	return f.lastWSConn
}
