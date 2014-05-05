package handlers_test

import (
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"regexp"
	"trafficcontroller/handlers"
)

var _ = Describe("HttpHandler", func() {
	var boundaryRegexp = regexp.MustCompile("boundary=(.*)")
	var handler http.Handler
	var fakeResponseWriter *httptest.ResponseRecorder
	var messagesChan chan []byte

	BeforeEach(func() {
		fakeResponseWriter = httptest.NewRecorder()
		messagesChan = make(chan []byte, 10)
		handler = handlers.NewHttpHandler(messagesChan, loggertesthelper.Logger())
	})

	It("grabs recent logs and creates a multi-part HTTP response", func(done Done) {
		r, _ := http.NewRequest("GET", "wss://loggregator.place/dump/?app=abc-123", nil)

		for i := 0; i < 5; i++ {
			messagesChan <- []byte("message")
		}

		close(messagesChan)
		handler.ServeHTTP(fakeResponseWriter, r)

		matches := boundaryRegexp.FindStringSubmatch(fakeResponseWriter.Header().Get("Content-Type"))
		Expect(matches).To(HaveLen(2))
		Expect(matches[1]).NotTo(BeEmpty())
		reader := multipart.NewReader(fakeResponseWriter.Body, matches[1])
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
		close(done)
	})

	//	Context("Multiple Loggregator Servers", func() {
	//		BeforeEach(func() {
	//			hashers = append(hashers, hasher.NewHasher([]string{"loggregator1", "loggregator2"}))
	//			fakeListeners = append(fakeListeners, trafficcontroller_testhelpers.NewFakeListener())
	//		})
	//
	//		It("handles hashing between loggregator servers", func() {
	//			r, _ := http.NewRequest("GET", "wss://loggregator.place/dump/?app=abc-123", nil)
	//
	//			correctAddress := hashers[0].GetLoggregatorServerForAppId("abc-123")
	//			handler.ServeHTTP(fakeResponseWriter, r)
	//			Expect(fakeListeners[0].Uri).To(Equal("ws://" + correctAddress + "/dump/?app=abc-123"))
	//		})
	//	})
	//
	//	Context("Multiple AZ, Multiple Loggregator Servers", func() {
	//		BeforeEach(func() {
	//			hashers = append(hashers, hasher.NewHasher([]string{"loggregator1"}), hasher.NewHasher([]string{"loggregator2"}))
	//			fakeListeners = append(fakeListeners, trafficcontroller_testhelpers.NewFakeListener())
	//		})
	//
	//		It("aggregates logs from multiple servers into one response", func() {
	//			r, _ := http.NewRequest("GET", "wss://loggregator.place/dump/?app=abc-123", nil)
	//
	//			fakeListeners[0].Channel <- []byte("message1")
	//			fakeListeners[1].Channel <- []byte("message2")
	//
	//			close(fakeListeners[0].Channel)
	//			close(fakeListeners[1].Channel)
	//			handler.ServeHTTP(fakeResponseWriter, r)
	//
	//			matches := boundaryRegexp.FindStringSubmatch(fakeResponseWriter.Header().Get("Content-Type"))
	//			Expect(matches).To(HaveLen(2))
	//			Expect(matches[1]).NotTo(BeEmpty())
	//			reader := multipart.NewReader(fakeResponseWriter.Body, matches[1])
	//			partsCount := 0
	//			var err error
	//			sawListener1, sawListener2 := false, false
	//			for err != io.EOF {
	//				var part *multipart.Part
	//				part, err = reader.NextPart()
	//				if err == io.EOF {
	//					break
	//				}
	//				partsCount++
	//
	//				data := make([]byte, 1024)
	//				n, _ := part.Read(data)
	//				Expect(n).To(BeNumerically(">", 0))
	//				if data[n-1] == '1' {
	//					sawListener1 = true
	//				} else if data[n-1] == '2' {
	//					sawListener2 = true
	//				} else {
	//					Fail("Saw strange message")
	//				}
	//			}
	//
	//			Expect(sawListener1).To(BeTrue())
	//			Expect(sawListener2).To(BeTrue())
	//			Expect(partsCount).To(Equal(2))
	//		})
	//	})

	It("sets the MIME type correctly", func() {
		close(messagesChan)
		r, err := http.NewRequest("GET", "", nil)
		Expect(err).ToNot(HaveOccurred())

		handler.ServeHTTP(fakeResponseWriter, r)
		Expect(fakeResponseWriter.Header().Get("Content-Type")).To(MatchRegexp(`multipart/x-protobuf; boundary=`))
	})
})
