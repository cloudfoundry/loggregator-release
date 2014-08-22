package dropsondeproxy_test

import (
	"fmt"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"

	"time"
	"trafficcontroller/dropsondeproxy"
	"trafficcontroller/listener"
	testhelpers "trafficcontroller_testhelpers"
)

var _ = Describe("DropsondeOutputProxySingleHasher", func() {

	var fwsh *fakeWebsocketHandler
	var fl *fakeListener
	var ts *httptest.Server
	var existingWsProvider = dropsondeproxy.NewWebsocketHandlerProvider

	BeforeEach(func() {
		fwsh = &fakeWebsocketHandler{}
		fl = &fakeListener{messageChan: make(chan []byte)}

		dropsondeproxy.NewWebsocketHandlerProvider = func(messageChan <-chan []byte) http.Handler {
			return fwsh
		}

		dropsondeproxy.NewWebsocketListener = func() listener.Listener {
			return fl
		}

		proxy := dropsondeproxy.NewDropsondeProxy(
			testhelpers.SuccessfulAuthorizer,
			cfcomponent.Config{},
			loggertesthelper.Logger(),
		)

		ts = httptest.NewServer(proxy)
		Eventually(serverUp(ts)).Should(BeTrue())
		dropsondeproxy.WebsocketKeepAliveDuration = time.Second
	})

	AfterEach(func() {
		ts.Close()
		dropsondeproxy.NewWebsocketHandlerProvider = existingWsProvider
	})

	Context("Auth Headers", func() {
		It("Should Authenticate with Correct Auth Header", func() {
			websocketClientWithHeaderAuth(ts, "/apps/myApp/stream", testhelpers.VALID_AUTHENTICATION_TOKEN)

			req := fwsh.lastRequest
			authenticateHeader := req.Header.Get("Authorization")
			Expect(authenticateHeader).To(Equal(testhelpers.VALID_AUTHENTICATION_TOKEN))

		})

		Context("when auth fails", func() {
			It("Should Fail to Authenticate with Incorrect Auth Header", func() {
				_, resp, err := websocketClientWithHeaderAuth(ts, "/apps/myApp/stream", testhelpers.INVALID_AUTHENTICATION_TOKEN)
				assertAuthorizationError(resp, err, "Error: Invalid authorization")
			})
		})

		Context("wrong URL path", func() {
			It("throws some kind of error if it has no 'apps' entry", func() {
				_, resp, err := websocketClientWithHeaderAuth(ts, "/OUPPS/myApp/stream", testhelpers.INVALID_AUTHENTICATION_TOKEN)
				assertResourceNotFoundError(resp, err, "Resource Not Found. /OUPPS/myApp/stream")
			})

			It("throws some kind of error if it has no appid", func() {
				_, resp, err := websocketClientWithHeaderAuth(ts, "/apps/stream", testhelpers.INVALID_AUTHENTICATION_TOKEN)
				assertResourceNotFoundError(resp, err, "Resource Not Found. /apps/stream")
			})

		})
	})

	Context("unknown endpoint", func() {
		It("returns an error", func() {
			_, resp, err := websocketClientWithHeaderAuth(ts, "/somewhereovertherainbow", testhelpers.INVALID_AUTHENTICATION_TOKEN)
			assertResourceNotFoundError(resp, err, "Resource Not Found. /somewhereovertherainbow")
		})
	})

	Context("/stream", func() {
		It("allows to connect over a websocket", func() {
			fl.SetExpectedHost("ws://localhost:62038/apps/myApp/stream")
			websocketClientWithHeaderAuth(ts, "/apps/myApp/stream", testhelpers.VALID_AUTHENTICATION_TOKEN)
			Expect(fwsh.called).To(BeTrue())
		})
	})

	Context("/tailinglogs", func() {
		It("allows to connect over a websocket", func() {
			fl.SetExpectedHost("ws://localhost:62038/apps/myApp/tailinglogs")
			websocketClientWithHeaderAuth(ts, "/apps/myApp/tailinglogs", testhelpers.VALID_AUTHENTICATION_TOKEN)
			Expect(fwsh.called).To(BeTrue())
		})
	})

	Context("/recentlogs", func() {
		var fhh *fakeHttpHandler
		var originalNewHttpHandlerProvider func(messages chan []byte) http.Handler

		BeforeEach(func() {
			fhh = &fakeHttpHandler{}
			fl.SetExpectedHost("ws://localhost:62038/apps/myApp/recentlogs")
			originalNewHttpHandlerProvider = dropsondeproxy.NewHttpHandlerProvider
			dropsondeproxy.NewHttpHandlerProvider = func(messageChan chan []byte) http.Handler {
				return fhh
			}
		})

		AfterEach(func() {
			dropsondeproxy.NewHttpHandlerProvider = originalNewHttpHandlerProvider
		})

		It("uses HttpHandler instead of WebsocketHandler", func() {
			url := fmt.Sprintf("http://%s/apps/myApp/recentlogs", ts.Listener.Addr())
			r, _ := http.NewRequest("GET", url, nil)
			r.Header = http.Header{"Authorization": []string{testhelpers.VALID_AUTHENTICATION_TOKEN}}
			client := &http.Client{}
			_, err := client.Do(r)

			Expect(err).NotTo(HaveOccurred())
			Expect(fhh.called).To(BeTrue())
		})
	})
})
