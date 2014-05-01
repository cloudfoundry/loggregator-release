package trafficcontroller_test

import (
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"trafficcontroller"
	"trafficcontroller/hasher"
	"trafficcontroller/listener"
	"trafficcontroller_testhelpers"
)

var _ = Describe("HttpHandler", func() {
	var handler *trafficcontroller.HttpHandler
	var fakeResponseWriter *httptest.ResponseRecorder
	var fakeListeners []*trafficcontroller_testhelpers.FakeListener
	var hashers []*hasher.Hasher

	var fakeListenerCounter int32
	trafficcontroller.NewWebsocketListener = func() listener.Listener {
		atomic.AddInt32(&fakeListenerCounter, 1)
		return fakeListeners[fakeListenerCounter-1]
	}

	BeforeEach(func() {
		fakeResponseWriter = httptest.NewRecorder()
		fakeListeners = []*trafficcontroller_testhelpers.FakeListener{trafficcontroller_testhelpers.NewFakeListener()}
		hashers = make([]*hasher.Hasher, 0)
		fakeListenerCounter = 0
	})

	JustBeforeEach(func() {
		handler = trafficcontroller.NewHttpHandler(hashers, loggertesthelper.Logger())
	})

	Context("One Loggregator Server", func() {
		BeforeEach(func() {
			hashers = append(hashers, hasher.NewHasher([]string{"loggregator1"}))
		})

		It("uses the Correct Request URI", func() {
			r, _ := http.NewRequest("GET", "wss://loggregator.place/dump/?app=abc-123", nil)
			close(fakeListeners[0].Channel)

			handler.ServeHTTP(fakeResponseWriter, r)
			Expect(fakeListeners[0].Uri).To(Equal("ws://loggregator1/dump/?app=abc-123"))
		})

		It("grabs recent logs and creates a multi-part HTTP response", func() {
			r, _ := http.NewRequest("GET", "wss://loggregator.place/dump/?app=abc-123", nil)

			for i := 0; i < 5; i++ {
				fakeListeners[0].Channel <- []byte("message")
			}

			close(fakeListeners[0].Channel)
			handler.ServeHTTP(fakeResponseWriter, r)

			reader := multipart.NewReader(fakeResponseWriter.Body, "loggregator-message")
			partsCount := 0
			var err error
			for err != io.EOF {
				var part *multipart.Part
				part, err = reader.NextPart()
				if err == io.EOF {
					break
				}
				partsCount++

				data := make([]byte, 1024)
				n, _ := part.Read(data)
				Expect(data[0:n]).To(Equal([]byte("message")))
			}

			Expect(partsCount).To(Equal(5))
		})
	})

	Context("Multiple Loggregator Servers", func() {
		BeforeEach(func() {
			hashers = append(hashers, hasher.NewHasher([]string{"loggregator1", "loggregator2"}))
			fakeListeners = append(fakeListeners, trafficcontroller_testhelpers.NewFakeListener())
		})

		It("handles hashing between loggregator servers", func() {
			r, _ := http.NewRequest("GET", "wss://loggregator.place/dump/?app=abc-123", nil)
			close(fakeListeners[0].Channel)

			correctAddress := hashers[0].GetLoggregatorServerForAppId("abc-123")
			handler.ServeHTTP(fakeResponseWriter, r)
			Expect(fakeListeners[0].Uri).To(Equal("ws://" + correctAddress + "/dump/?app=abc-123"))
		})
	})

	Context("Multiple AZ, Multiple Loggregator Servers", func() {
		BeforeEach(func() {
			hashers = append(hashers, hasher.NewHasher([]string{"loggregator1"}), hasher.NewHasher([]string{"loggregator2"}))
			fakeListeners = append(fakeListeners, trafficcontroller_testhelpers.NewFakeListener())
		})

		It("aggregates logs from multiple servers into one response", func() {
			r, _ := http.NewRequest("GET", "wss://loggregator.place/dump/?app=abc-123", nil)

			fakeListeners[0].Channel <- []byte("message1")
			fakeListeners[1].Channel <- []byte("message2")

			close(fakeListeners[0].Channel)
			close(fakeListeners[1].Channel)
			handler.ServeHTTP(fakeResponseWriter, r)

			reader := multipart.NewReader(fakeResponseWriter.Body, "loggregator-message")
			partsCount := 0
			var err error
			sawListener1, sawListener2 := false, false
			for err != io.EOF {
				var part *multipart.Part
				part, err = reader.NextPart()
				if err == io.EOF {
					break
				}
				partsCount++

				data := make([]byte, 1024)
				n, _ := part.Read(data)
				Expect(n).To(BeNumerically(">", 0))
				if data[n-1] == '1' {
					sawListener1 = true
				} else if data[n-1] == '2' {
					sawListener2 = true
				} else {
					Fail("Saw strange message")
				}
			}

			Expect(sawListener1).To(BeTrue())
			Expect(sawListener2).To(BeTrue())
			Expect(partsCount).To(Equal(2))
		})
	})

	It("sets the MIME type correctly", func() {
		r, err := http.NewRequest("GET", "", nil)
		Expect(err).ToNot(HaveOccurred())

		handler.ServeHTTP(fakeResponseWriter, r)
		Expect(fakeResponseWriter.Header().Get("Content-Type")).To(Equal(`multipart/x-protobuf; boundary="loggregator-message"`))
	})
})
