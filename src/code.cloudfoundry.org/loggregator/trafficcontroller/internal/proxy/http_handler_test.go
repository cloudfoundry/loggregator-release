package proxy_test

import (
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"regexp"

	"code.cloudfoundry.org/loggregator/trafficcontroller/internal/proxy"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("HttpHandler", func() {
	var boundaryRegexp = regexp.MustCompile("boundary=(.*)")
	var handler http.Handler
	var fakeResponseWriter *httptest.ResponseRecorder
	var messagesChan chan []byte

	BeforeEach(func() {
		fakeResponseWriter = httptest.NewRecorder()
		messagesChan = make(chan []byte, 10)
		handler = proxy.NewHttpHandler(messagesChan)
	})

	It("grabs recent logs and creates a multi-part HTTP response", func(done Done) {
		r, _ := http.NewRequest("GET", "ws://loggregator.place/dump/?app=abc-123", nil)

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

	It("sets the MIME type correctly", func() {
		close(messagesChan)
		r, err := http.NewRequest("GET", "", nil)
		Expect(err).ToNot(HaveOccurred())

		handler.ServeHTTP(fakeResponseWriter, r)
		Expect(fakeResponseWriter.Header().Get("Content-Type")).To(MatchRegexp(`multipart/x-protobuf; boundary=`))
	})
})
