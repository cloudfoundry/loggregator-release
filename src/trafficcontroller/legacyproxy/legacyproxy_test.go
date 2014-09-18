package legacyproxy_test

import (
	"trafficcontroller/legacyproxy"

	"errors"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"
)

var _ = Describe("Legacyproxy", func() {

	Context("request delegation", func() {
		It("delegates to the wrapped handler", func() {
			proxyBuilder := legacyproxy.NewProxyBuilder()
			request, _ := http.NewRequest("GET", "rabbits", nil)
			fakeTranslator := &fakeTranslator{translatedRequest: request}
			fakeHandler := &fakeHandler{}

			responseWriter := httptest.NewRecorder()
			proxy := proxyBuilder.RequestTranslator(fakeTranslator).Handler(fakeHandler).Build()
			proxy.ServeHTTP(responseWriter, nil)

			Expect(fakeHandler.writer).To(Equal(responseWriter))
			Expect(fakeHandler.request).To(Equal(request))
		})
	})

	Context("Request Translation", func() {
		It("returns a 400 error if the request is invalid", func() {
			proxyBuilder := legacyproxy.NewProxyBuilder()
			fakeError := errors.New("fake error")
			fakeTranslator := &fakeTranslator{fakeError: fakeError}

			request, _ := http.NewRequest("GET", "rabbits", nil)
			responseWriter := httptest.NewRecorder()
			proxy := proxyBuilder.RequestTranslator(fakeTranslator).Handler(&fakeHandler{}).Build()
			proxy.ServeHTTP(responseWriter, request)

			Expect(responseWriter.Code).To(Equal(400))
			Expect(responseWriter.Body.String()).To(ContainSubstring("invalid request: fake error"))
		})
	})
})

type fakeTranslator struct {
	wasCalled         bool
	fakeError         error
	translatedRequest *http.Request
}

func (pt *fakeTranslator) Translate(request *http.Request) (*http.Request, error) {
	pt.wasCalled = true
	return pt.translatedRequest, pt.fakeError
}

type fakeHandler struct {
	writer  http.ResponseWriter
	request *http.Request
}

func (fh *fakeHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	fh.writer = writer
	fh.request = request
}
