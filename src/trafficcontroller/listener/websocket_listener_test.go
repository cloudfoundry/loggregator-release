package listener_test

import (
	"fmt"
	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"log"
	"net/http"
	"net/http/httptest"
	"trafficcontroller/listener"
)

type fakeHandler struct {
	messages <-chan []byte
}

func (f fakeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	ws, err := websocket.Upgrade(w, r, nil, 1024, 1024)
	if _, ok := err.(websocket.HandshakeError); ok {
		http.Error(w, "Not a websocket handshake", 400)
		return
	} else if err != nil {
		log.Println(err)
		return
	}

	for msg := range f.messages {
		if err := ws.WriteMessage(websocket.BinaryMessage, msg); err != nil {
			return
		}
	}
}

var _ = Describe("WebsocketListener", func() {

	var ts *httptest.Server
	var messageChan chan []byte
	var l listener.Listener

	BeforeEach(func() {
		messageChan = make(chan []byte)
		ts = httptest.NewUnstartedServer(fakeHandler{messageChan})
		l = listener.NewWebsocket()
	})

	AfterEach(func() {
		close(messageChan)
		ts.Close()
	})

	Context("when the server is not running", func() {
		It("should error when connecting", func() {
			outputChan, err := l.Start("ws://localhost:1234")
			Expect(err).To(HaveOccurred())
			Expect(outputChan).To(BeClosed())
		})
	})

	Context("when the server is running", func() {
		BeforeEach(func() {
			ts.Start()
		})

		It("should connect to a websocket", func() {
			outputChan, err := l.Start(fmt.Sprintf("ws://%s", ts.Listener.Addr().String()))
			Expect(err).NotTo(HaveOccurred())
			Expect(outputChan).NotTo(BeClosed())
		})

		It("should output messages recieved from the server", func(done Done) {
			outputChan, _ := l.Start(fmt.Sprintf("ws://%s", ts.Listener.Addr().String()))

			message := []byte("hello world")
			messageChan <- message

			var receivedMessage []byte
			Eventually(outputChan).Should(Receive(&receivedMessage))
			Expect(receivedMessage).To(Equal(message))

			close(done)
		})

		It("should close the channel when server returns an error", func() {
			outputChan, _ := l.Start(fmt.Sprintf("ws://%s", ts.Listener.Addr().String()))
			ts.CloseClientConnections()
			Eventually(outputChan).Should(BeClosed())
		})
	})
})
