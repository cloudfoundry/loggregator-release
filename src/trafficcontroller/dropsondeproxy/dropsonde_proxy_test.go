package dropsondeproxy_test

import (
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"

	"github.com/cloudfoundry/loggregatorlib/server/handlers"
	"time"
	"trafficcontroller/dropsondeproxy"
	testhelpers "trafficcontroller_testhelpers"
)

var _ = Describe("ServeHTTP", func() {
	var (
		auth     testhelpers.Authorizer
		proxy    *dropsondeproxy.Proxy
		recorder *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		auth = testhelpers.Authorizer{Result: true}

		proxy = dropsondeproxy.NewDropsondeProxy(
			auth.Authorize,
			cfcomponent.Config{},
			loggertesthelper.Logger(),
		)

		recorder = httptest.NewRecorder()
	})

	It("returns a 200 for a head request", func() {
		req, _ := http.NewRequest("HEAD", "", nil)

		proxy.ServeHTTP(recorder, req)

		Expect(recorder.Code).To(Equal(http.StatusOK))
	})

	It("returns a 404 and sets the WWW-Authenticate to basic if the path does not start with /apps", func() {
		req, _ := http.NewRequest("GET", "/notApps", nil)

		proxy.ServeHTTP(recorder, req)

		Expect(recorder.Code).To(Equal(http.StatusNotFound))
		Expect(recorder.HeaderMap.Get("WWW-Authenticate")).To(Equal("Basic"))
		Expect(recorder.Body.String()).To(Equal("Resource Not Found. /notApps"))
	})

	It("returns a 404 and sets the WWW-Authenticate to basic if the path does not end with /stream or /recentlogs", func() {
		req, _ := http.NewRequest("GET", "/apps/abc123/bar", nil)

		proxy.ServeHTTP(recorder, req)

		Expect(recorder.Code).To(Equal(http.StatusNotFound))
		Expect(recorder.HeaderMap.Get("WWW-Authenticate")).To(Equal("Basic"))
		Expect(recorder.Body.String()).To(Equal("Resource Not Found. /apps/abc123/bar"))
	})

	It("returns a 404 and sets the WWW-Authenticate to basic if the app id is missing", func() {
		req, _ := http.NewRequest("GET", "/apps//stream", nil)

		proxy.ServeHTTP(recorder, req)

		Expect(recorder.Code).To(Equal(http.StatusNotFound))
		Expect(recorder.HeaderMap.Get("WWW-Authenticate")).To(Equal("Basic"))
		Expect(recorder.Body.String()).To(Equal("App ID missing. Make request to /apps/APP_ID/stream"))
	})

	It("returns a unauthorized status and sets the WWW-Authenticate header if auth not provided", func() {
		req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)

		proxy.ServeHTTP(recorder, req)

		Expect(recorder.Code).To(Equal(http.StatusUnauthorized))
		Expect(recorder.HeaderMap.Get("WWW-Authenticate")).To(Equal("Basic"))
		Expect(recorder.Body.String()).To(Equal("You are not authorized. Error: Authorization not provided"))
	})

	It("returns an unauthorized status and sets the WWW-Authenticate header if authorization fails", func() {
		auth.Result = false

		req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)
		req.Header.Add("Authorization", "token")

		proxy.ServeHTTP(recorder, req)

		Expect(auth.TokenParam).To(Equal("token"))
		Expect(auth.Target).To(Equal("abc123"))

		Expect(recorder.Code).To(Equal(http.StatusUnauthorized))
		Expect(recorder.HeaderMap.Get("WWW-Authenticate")).To(Equal("Basic"))
		Expect(recorder.Body.String()).To(Equal("You are not authorized. Error: Invalid authorization"))
	})

	It("can read the authorization information from a cookie", func() {
		auth.Result = false

		req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)

		req.AddCookie(&http.Cookie{Name: "authorization", Value: "cookie-token"})

		proxy.ServeHTTP(recorder, req)

		Expect(auth.TokenParam).To(Equal("cookie-token"))
	})

	It("uses the handler provided to serve http", func() {
		req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)
		req.Header.Add("Authorization", "token")

		fakeHandler := struct {
			Endpoint string
			Messages <-chan []byte
		}{}

		fakeHttpHandler := &fakeHttpHandler{}

		oldHandlerProvider := dropsondeproxy.HandlerProvider
		dropsondeproxy.HandlerProvider = func(endpoint string, messages <-chan []byte) http.Handler {
			fakeHandler.Endpoint = endpoint
			fakeHandler.Messages = messages
			return fakeHttpHandler
		}

		proxy.ServeHTTP(recorder, req)

		Expect(fakeHandler.Endpoint).To(Equal("stream"))
		Expect(fakeHandler.Messages).ToNot(BeNil())

		Expect(fakeHttpHandler.called).To(BeTrue())
		dropsondeproxy.HandlerProvider = oldHandlerProvider
	})
})

var _ = Describe("HandlerProvider", func() {
	It("returns an HTTP handler for .../recentlogs", func() {
		httpHandler := handlers.NewHttpHandler(make(chan []byte))

		target := dropsondeproxy.HandlerProvider("recentlogs", make(chan []byte))

		Expect(target).To(BeAssignableToTypeOf(httpHandler))
	})

	It("returns a Websocket handler for .../stream", func() {
		wsHandler := handlers.NewWebsocketHandler(make(chan []byte), time.Minute)

		target := dropsondeproxy.HandlerProvider("stream", make(chan []byte))

		Expect(target).To(BeAssignableToTypeOf(wsHandler))
	})

	It("returns a Websocket handler for anything else", func() {
		wsHandler := handlers.NewWebsocketHandler(make(chan []byte), time.Minute)

		target := dropsondeproxy.HandlerProvider("other", make(chan []byte))

		Expect(target).To(BeAssignableToTypeOf(wsHandler))
	})
})
