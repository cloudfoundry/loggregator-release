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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServeHTTP", func() {
	var (
		auth          testhelpers.Authorizer
		proxy         *dopplerproxy.Proxy
		recorder      *httptest.ResponseRecorder
		fakeHandler   *fakeHttpHandler
		fakeConnector *fakeChannelGroupConnector
	)

	var fakeHandlerProvider = func(endpoint string, messages <-chan []byte) http.Handler {
		fakeHandler.endpoint = endpoint
		fakeHandler.messages = messages
		return fakeHandler
	}

	BeforeEach(func() {
		auth = testhelpers.Authorizer{Result: true}

		fakeHandler = &fakeHttpHandler{}
		fakeConnector = &fakeChannelGroupConnector{messages: make(chan []byte, 10)}

		proxy = dopplerproxy.NewDopplerProxy(
			auth.Authorize,
			fakeHandlerProvider,
			fakeConnector,
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

		proxy.ServeHTTP(recorder, req)

		Expect(fakeHandler.endpoint).To(Equal("stream"))
		Expect(fakeHandler.messages).ToNot(BeNil())

		Expect(fakeHandler.called).To(BeTrue())
	})

	It("connects to doppler servers with correct parameters", func() {
		req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)
		req.Header.Add("Authorization", "token")

		proxy.ServeHTTP(recorder, req)

		Eventually(fakeConnector.getPath).Should(Equal("/stream"))
		Eventually(fakeConnector.getAppId).Should(Equal("abc123"))
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

var _ = Describe("DefaultHandlerProvider", func() {
	It("returns an HTTP handler for .../recentlogs", func() {
		httpHandler := handlers.NewHttpHandler(make(chan []byte))

		target := dopplerproxy.DefaultHandlerProvider("recentlogs", make(chan []byte))

		Expect(target).To(BeAssignableToTypeOf(httpHandler))
	})

	It("returns a Websocket handler for .../stream", func() {
		wsHandler := handlers.NewWebsocketHandler(make(chan []byte), time.Minute)

		target := dopplerproxy.DefaultHandlerProvider("stream", make(chan []byte))

		Expect(target).To(BeAssignableToTypeOf(wsHandler))
	})

	It("returns a Websocket handler for anything else", func() {
		wsHandler := handlers.NewWebsocketHandler(make(chan []byte), time.Minute)

		target := dopplerproxy.DefaultHandlerProvider("other", make(chan []byte))

		Expect(target).To(BeAssignableToTypeOf(wsHandler))
	})
})

type fakeChannelGroupConnector struct {
	messages  chan []byte
	path      string
	appId     string
	reconnect bool
	stopped   bool
	sync.Mutex
}

func (f *fakeChannelGroupConnector) Connect(path string, appId string, messagesChan chan<- []byte, stopChan <-chan struct{}, reconnect bool) {

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
	f.path = path
	f.appId = appId
	f.reconnect = reconnect
}

func (f *fakeChannelGroupConnector) getPath() string {
	f.Lock()
	defer f.Unlock()
	return f.path
}

func (f *fakeChannelGroupConnector) getAppId() string {
	f.Lock()
	defer f.Unlock()
	return f.appId
}

func (f *fakeChannelGroupConnector) getReconnect() bool {
	f.Lock()
	defer f.Unlock()
	return f.reconnect
}

func (f *fakeChannelGroupConnector) Stopped() bool {
	f.Lock()
	defer f.Unlock()
	return f.stopped
}
