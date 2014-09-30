package dopplerproxy_test

import (
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/server/handlers"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"
	"trafficcontroller/dopplerproxy"
	testhelpers "trafficcontroller_testhelpers"

	"github.com/cloudfoundry/gosteno"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"trafficcontroller/doppler_endpoint"
)

var _ = Describe("ServeHTTP", func() {
	var (
		auth          testhelpers.LogAuthorizer
		adminAuth     testhelpers.AdminAuthorizer
		proxy         *dopplerproxy.Proxy
		recorder      *httptest.ResponseRecorder
		fakeHandler   *fakeHttpHandler
		fakeConnector *fakeChannelGroupConnector
	)

	var fakeHandlerProvider = func(messages <-chan []byte, logger *gosteno.Logger) http.Handler {
		fakeHandler.messages = messages
		return fakeHandler
	}

	BeforeEach(func() {
		auth = testhelpers.LogAuthorizer{Result: true}
		adminAuth = testhelpers.AdminAuthorizer{Result: true}

		fakeHandler = &fakeHttpHandler{}
		fakeConnector = &fakeChannelGroupConnector{messages: make(chan []byte, 10)}

		proxy = dopplerproxy.NewDopplerProxy(
			auth.Authorize,
			adminAuth.Authorize,
			fakeHandlerProvider,
			fakeConnector,
			cfcomponent.Config{},
			loggertesthelper.Logger(),
		)

		recorder = httptest.NewRecorder()
	})

	Context("App Logs", func() {

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

			proxy.ServeHTTP(recorder, req)

			Expect(fakeHandler.messages).ToNot(BeNil())

			Expect(fakeHandler.called).To(BeTrue())
		})

		It("connects to doppler servers with correct parameters", func() {
			req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)
			req.Header.Add("Authorization", "token")

			proxy.ServeHTTP(recorder, req)

			Eventually(fakeConnector.getPath).Should(Equal("stream"))
			Eventually(fakeConnector.getReconnect).Should(BeTrue())
		})

		It("connects to doppler servers without reconnecting for recentlogs", func() {
			req, _ := http.NewRequest("GET", "/apps/abc123/recentlogs", nil)
			req.Header.Add("Authorization", "token")

			proxy.ServeHTTP(recorder, req)

			Eventually(fakeConnector.getReconnect).Should(BeFalse())
		})

		It("connects to doppler servers and passes their messages to the handler", func() {
			req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)
			req.Header.Add("Authorization", "token")

			proxy.ServeHTTP(recorder, req)
			fakeConnector.messages <- []byte("hello")

			Eventually(fakeHandler.messages).Should(Receive(BeEquivalentTo("hello")))
		})

		It("stops the connector when the handler finishes", func() {
			req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)
			req.Header.Add("Authorization", "token")

			proxy.ServeHTTP(recorder, req)

			Eventually(fakeConnector.Stopped).Should(BeTrue())
		})
	})

	Context("Firehose", func() {
		It("connects to doppler servers with correct parameters", func() {
			req, _ := http.NewRequest("GET", "/firehose", nil)
			req.Header.Add("Authorization", "token")

			proxy.ServeHTTP(recorder, req)

			Eventually(fakeConnector.getPath).Should(Equal("firehose"))
			Eventually(fakeConnector.getAppId).Should(Equal("firehose"))
			Eventually(fakeConnector.getReconnect).Should(BeTrue())
		})

		It("returns an unauthorized status and sets the WWW-Authenticate header if authorization fails", func() {
			adminAuth.Result = false

			req, _ := http.NewRequest("GET", "/firehose", nil)
			req.Header.Add("Authorization", "token")

			proxy.ServeHTTP(recorder, req)

			Expect(adminAuth.TokenParam).To(Equal("token"))

			Expect(recorder.Code).To(Equal(http.StatusUnauthorized))
			Expect(recorder.HeaderMap.Get("WWW-Authenticate")).To(Equal("Basic"))
			Expect(recorder.Body.String()).To(Equal("You are not authorized. Error: Invalid authorization"))
		})

		It("returns an unauthorized status and sets the WWW-Authenticate header if the token is blank", func() {
			adminAuth.Result = true

			req, _ := http.NewRequest("GET", "/firehose", nil)
			req.Header.Add("Authorization", "")

			proxy.ServeHTTP(recorder, req)

			Expect(adminAuth.TokenParam).To(Equal(""))

			Expect(recorder.Code).To(Equal(http.StatusUnauthorized))
			Expect(recorder.HeaderMap.Get("WWW-Authenticate")).To(Equal("Basic"))
			Expect(recorder.Body.String()).To(Equal("You are not authorized. Error: Authorization not provided"))
		})
	})
})

var _ = Describe("DefaultHandlerProvider", func() {
	It("returns an HTTP handler for .../recentlogs", func() {
		httpHandler := handlers.NewHttpHandler(make(chan []byte), loggertesthelper.Logger())

		target := doppler_endpoint.HttpHandlerProvider(make(chan []byte), loggertesthelper.Logger())

		Expect(target).To(BeAssignableToTypeOf(httpHandler))
	})

	It("returns a Websocket handler for .../stream", func() {
		wsHandler := handlers.NewWebsocketHandler(make(chan []byte), time.Minute, loggertesthelper.Logger())

		target := doppler_endpoint.WebsocketHandlerProvider(make(chan []byte), loggertesthelper.Logger())

		Expect(target).To(BeAssignableToTypeOf(wsHandler))
	})

	It("returns a Websocket handler for anything else", func() {
		wsHandler := handlers.NewWebsocketHandler(make(chan []byte), time.Minute, loggertesthelper.Logger())

		target := doppler_endpoint.WebsocketHandlerProvider(make(chan []byte), loggertesthelper.Logger())

		Expect(target).To(BeAssignableToTypeOf(wsHandler))
	})
})

type fakeChannelGroupConnector struct {
	messages        chan []byte
	dopplerEndpoint doppler_endpoint.DopplerEndpoint
	stopped         bool
	sync.Mutex
}

func (f *fakeChannelGroupConnector) Connect(dopplerEndpoint doppler_endpoint.DopplerEndpoint, messagesChan chan<- []byte, stopChan <-chan struct{}) {

	go func() {
		for m := range f.messages {
			messagesChan <- m
		}
	}()

	go func() {
		<-stopChan
		f.Lock()
		defer f.Unlock()
		f.stopped = true
	}()

	f.Lock()
	defer f.Unlock()
	f.dopplerEndpoint = dopplerEndpoint
}

func (f *fakeChannelGroupConnector) getPath() string {
	f.Lock()
	defer f.Unlock()
	return f.dopplerEndpoint.Endpoint
}

func (f *fakeChannelGroupConnector) getAppId() string {
	f.Lock()
	defer f.Unlock()
	return f.dopplerEndpoint.StreamId
}

func (f *fakeChannelGroupConnector) getReconnect() bool {
	f.Lock()
	defer f.Unlock()
	return f.dopplerEndpoint.Reconnect
}

func (f *fakeChannelGroupConnector) Stopped() bool {
	f.Lock()
	defer f.Unlock()
	return f.stopped
}
