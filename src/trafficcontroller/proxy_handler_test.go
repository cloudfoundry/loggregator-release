package trafficcontroller

import (
	"errors"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"
	"time"
	"trafficcontroller/hasher"
)

type fakeLoggregator struct {
	messages         []string
	receivedRequests []*http.Request
	numKeepAlives    int
}

func (fl *fakeLoggregator) ReceivedRequests() []*http.Request {
	return fl.receivedRequests
}

func (fl *fakeLoggregator) ReceivedKeepAlives() int {
	return fl.numKeepAlives
}

func (fl *fakeLoggregator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fl.receivedRequests = append(fl.receivedRequests, r)
	ws, err := websocket.Upgrade(w, r, nil, 1024, 1024)
	if _, ok := err.(websocket.HandshakeError); ok {
		Fail("Bad Handshake")
		return
	} else if err != nil {
		Fail("Upgrade Close")
		return
	}
	defer ws.Close()

	r.ParseForm()

	for _, msg := range fl.messages {
		ws.WriteMessage(websocket.BinaryMessage, []byte(msg))
	}
	for {
		_, _, err := ws.ReadMessage()

		if err == nil {
			fl.numKeepAlives++
			continue
		}
		return
	}
}

type fakeClient struct {
	receivedMessages     []string
	receivedMessageTypes []int
	keepAlives           int
}

func (f *fakeClient) WriteMessage(messageType int, data []byte) error {
	f.receivedMessages = append(f.receivedMessages, string(data))
	f.receivedMessageTypes = append(f.receivedMessageTypes, messageType)

	return nil
}

func (f *fakeClient) ReadMessage() (int, []byte, error) {
	if f.keepAlives == 0 {
		return 0, []byte(""), errors.New("EOF")
	}
	f.keepAlives--
	return 0, []byte("Keep Alive"), nil
}

func (f *fakeClient) WriteControl(int, []byte, time.Time) error {
	return nil
}

func (f *fakeClient) ReceivedMessages() []string {
	return f.receivedMessages
}

func (f *fakeClient) ReceivedMessageTypes() []int {
	return f.receivedMessageTypes
}

var _ = Describe("ProxyHandler", func() {

	Context("One Loggregator Server", func() {

		var fakeServer *httptest.Server
		var loggregator *fakeLoggregator

		BeforeEach(func() {
			loggregator = &fakeLoggregator{messages: []string{"Message1", "Message2"}}
			fakeServer = httptest.NewServer(loggregator)
		})

		AfterEach(func() {
			fakeServer.Close()
		})

		It("Proxies multiple messages", func() {
			fakeClient := &fakeClient{}
			handler := NewProxyHandler(
				fakeClient,
				loggertesthelper.Logger(),
			)

			handler.HandleWebSocket(
				"appId",
				"/dump",
				[]*hasher.Hasher{hasher.NewHasher([]string{fakeServer.Listener.Addr().String()})})

			Expect(fakeClient.ReceivedMessageTypes()).To(HaveLen(2))
			Expect(fakeClient.ReceivedMessages()).To(Equal([]string{"Message1", "Message2"}))
			Expect(fakeClient.ReceivedMessageTypes()).To(ContainElement(websocket.BinaryMessage))

		})

		It("Uses the Correct Request URI", func() {
			handler := NewProxyHandler(
				&fakeClient{},
				loggertesthelper.Logger(),
			)

			handler.HandleWebSocket(
				"appId",
				"/dump",
				[]*hasher.Hasher{hasher.NewHasher([]string{fakeServer.Listener.Addr().String()})})

			Expect(loggregator.ReceivedRequests()).To(HaveLen(1))
			Expect(loggregator.ReceivedRequests()[0].URL.Path).To(Equal("/dump"))
		})

		It("Forwards KeepAlives", func() {
			handler := NewProxyHandler(
				&fakeClient{keepAlives: 5},
				loggertesthelper.Logger(),
			)

			handler.HandleWebSocket(
				"appId",
				"/dump",
				[]*hasher.Hasher{hasher.NewHasher([]string{fakeServer.Listener.Addr().String()})})

			Expect(loggregator.ReceivedKeepAlives()).Should(Equal(5))
		})
	})
	Context("Multiple Loggregator Server", func() {
		It("It handles hashing between loggregator servers", func() {
			s1 := httptest.NewServer(&fakeLoggregator{messages: []string{"loggregator1:a"}})
			s2 := httptest.NewServer(&fakeLoggregator{messages: []string{"loggregator2:a"}})
			defer s1.Close()
			defer s2.Close()
			fakeClient := &fakeClient{}
			handler := NewProxyHandler(
				fakeClient,
				loggertesthelper.Logger(),
			)

			loggregators := []string{s1.Listener.Addr().String(), s2.Listener.Addr().String()}

			handler.HandleWebSocket(
				"appId",
				"/dump",
				[]*hasher.Hasher{hasher.NewHasher(loggregators)})

			Expect(fakeClient.ReceivedMessages()).To(HaveLen(1))
			Expect(fakeClient.ReceivedMessageTypes()).To(HaveLen(1))
			Expect(fakeClient.ReceivedMessages()).To(ContainElement("loggregator1:a"))
			Expect(fakeClient.ReceivedMessageTypes()).To(ContainElement(websocket.BinaryMessage))
		})
	})
	Context("Multiple AZ, Multiple Loggregator Server", func() {
		var fakeLoggregators [4]*fakeLoggregator
		var fakeServers [4]*httptest.Server
		var hashersAZ1 *hasher.Hasher
		var hashersAZ2 *hasher.Hasher

		BeforeEach(func() {
			fakeLoggregators[0] = &fakeLoggregator{messages: []string{"loggregator:AZ1:SERVER1"}}
			fakeLoggregators[1] = &fakeLoggregator{messages: []string{"loggregator:AZ1:SERVER2"}}
			fakeLoggregators[2] = &fakeLoggregator{messages: []string{"loggregator:AZ2:SERVER1"}}
			fakeLoggregators[3] = &fakeLoggregator{messages: []string{"loggregator:AZ2:SERVER2"}}

			for i, loggregator := range fakeLoggregators {
				fakeServers[i] = httptest.NewServer(loggregator)
			}
			hashersAZ1 = hasher.NewHasher([]string{fakeServers[0].Listener.Addr().String(), fakeServers[1].Listener.Addr().String()})
			hashersAZ2 = hasher.NewHasher([]string{fakeServers[2].Listener.Addr().String(), fakeServers[3].Listener.Addr().String()})

		})
		AfterEach(func() {
			for _, server := range fakeServers {
				server.Close()
			}
		})
		It("Proxies messages", func() {
			fakeClient := &fakeClient{}
			handler := NewProxyHandler(
				fakeClient,
				loggertesthelper.Logger(),
			)

			handler.HandleWebSocket(
				"appId",
				"/dump",
				[]*hasher.Hasher{hashersAZ1, hashersAZ2})

			Expect(fakeClient.ReceivedMessages()).To(HaveLen(2))
			Expect(fakeClient.ReceivedMessageTypes()).To(HaveLen(2))
			Expect(fakeClient.ReceivedMessages()).To(ContainElement("loggregator:AZ1:SERVER1"))
			Expect(fakeClient.ReceivedMessages()).To(ContainElement("loggregator:AZ2:SERVER1"))
			Expect(fakeClient.ReceivedMessageTypes()).To(ContainElement(websocket.BinaryMessage))
		})

		It("Forwards KeepAlives", func() {
			handler := NewProxyHandler(
				&fakeClient{keepAlives: 3},
				loggertesthelper.Logger(),
			)

			handler.HandleWebSocket(
				"appId",
				"/dump",
				[]*hasher.Hasher{hashersAZ1, hashersAZ2})

			Expect(fakeLoggregators[0].ReceivedKeepAlives()).Should(Equal(3))
			Expect(fakeLoggregators[1].ReceivedKeepAlives()).Should(Equal(0))
			Expect(fakeLoggregators[2].ReceivedKeepAlives()).Should(Equal(3))
			Expect(fakeLoggregators[3].ReceivedKeepAlives()).Should(Equal(0))
		})

	})
})
