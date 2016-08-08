package logcounter_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"tools/logcounterapp/config"
	"tools/logcounterapp/logcounter"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("logCounter", func() {
	Describe("Start", func() {
		It("returns 200 status for dumpReport", func() {
			var (
				mu       sync.Mutex
				request  *http.Request
				statuses = make(chan int, 2)
			)
			statuses <- http.StatusOK
			statuses <- http.StatusCreated

			handler := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(<-statuses)
				mu.Lock()
				defer mu.Unlock()
				request = r
			}
			testServer := httptest.NewServer(http.HandlerFunc(handler))
			defer func() {
				Eventually(func() string {
					mu.Lock()
					defer mu.Unlock()
					if request == nil {
						return ""
					}
					return request.URL.Path
				}).Should(Equal("/report"))
				testServer.Close()
			}()

			port := testPort()

			cc := newMockCC()
			uaa := newMockUAA()
			cfg := conf(port, testServer)

			lc := logcounter.New(uaa, cc, cfg)
			go func() {
				defer GinkgoRecover()
				err := lc.Start()
				Expect(err).ToNot(HaveOccurred())
			}()
			defer lc.Stop()

			Eventually(func() bool { return checkEndpoint(port, "?report", http.StatusOK) }).Should(BeTrue())
		})

		It("returns 404 status for unknown routes", func() {
			var (
				mu       sync.Mutex
				request  *http.Request
				statuses = make(chan int, 2)
			)
			statuses <- http.StatusOK
			statuses <- http.StatusCreated

			handler := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(<-statuses)
				mu.Lock()
				defer mu.Unlock()
				request = r
			}
			testServer := httptest.NewServer(http.HandlerFunc(handler))
			defer func() {
				Eventually(func() string {
					mu.Lock()
					defer mu.Unlock()
					if request == nil {
						return ""
					}
					return request.URL.Path
				}).Should(Equal("/report"))
				testServer.Close()
			}()

			port := testPort()

			cc := newMockCC()
			uaa := newMockUAA()
			cfg := conf(port, testServer)

			lc := logcounter.New(uaa, cc, cfg)
			go func() {
				defer GinkgoRecover()
				err := lc.Start()
				Expect(err).ToNot(HaveOccurred())
			}()
			defer lc.Stop()

			Eventually(func() bool { return checkEndpoint(port, "doesntExist", http.StatusNotFound) }).Should(BeTrue())

		})
	})

	Describe("Stop", func() {
		It("stops accepting requests", func() {
			var (
				mu       sync.Mutex
				request  *http.Request
				statuses = make(chan int, 2)
			)
			statuses <- http.StatusOK
			statuses <- http.StatusCreated

			handler := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(<-statuses)
				mu.Lock()
				defer mu.Unlock()
				request = r
			}
			testServer := httptest.NewServer(http.HandlerFunc(handler))
			defer func() {
				Eventually(func() string {
					mu.Lock()
					defer mu.Unlock()
					if request == nil {
						return ""
					}
					return request.URL.Path
				}).Should(Equal("/report"))
				testServer.Close()
			}()

			port := testPort()

			cc := newMockCC()
			uaa := newMockUAA()
			cfg := conf(port, testServer)

			lc := logcounter.New(uaa, cc, cfg)
			go func() {
				defer GinkgoRecover()
				err := lc.Start()
				Expect(err).ToNot(HaveOccurred())
			}()

			Eventually(func() bool { return checkEndpoint(port, "?report", http.StatusOK) }).Should(BeTrue())

			lc.Stop()
			Eventually(func() bool { return checkEndpoint(port, "?report", http.StatusOK) }).Should(BeFalse())
		})
	})

	Describe("HandleMessages", func() {
		It("results in a correct report when envelopes are passed to it", func() {
			var (
				mu       sync.Mutex
				request  *http.Request
				statuses = make(chan int, 2)
			)
			statuses <- http.StatusOK
			statuses <- http.StatusCreated

			handler := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(<-statuses)
				mu.Lock()
				defer mu.Unlock()
				request = r
			}
			testServer := httptest.NewServer(http.HandlerFunc(handler))
			defer func() {
				Eventually(func() string {
					mu.Lock()
					defer mu.Unlock()
					if request == nil {
						return ""
					}
					return request.URL.Path
				}).Should(Equal("/report"))
				testServer.Close()
			}()

			port := testPort()

			cc := newMockCC()
			uaa := newMockUAA()
			cfg := conf(port, testServer)

			lc := logcounter.New(uaa, cc, cfg)
			go func() {
				defer GinkgoRecover()
				err := lc.Start()
				Expect(err).ToNot(HaveOccurred())
			}()

			Eventually(func() bool { return checkEndpoint(port, "?report", http.StatusOK) }).Should(BeTrue())

			msgs := make(chan *events.Envelope, 10)
			for i := 0; i < 10; i++ {
				envelope := &events.Envelope{
					Origin:    proto.String("testOrigin"),
					EventType: events.Envelope_LogMessage.Enum(),
					LogMessage: &events.LogMessage{
						Message:     []byte(fmt.Sprintf("testPrefix guid: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx msg: %d", i)),
						MessageType: events.LogMessage_OUT.Enum(),
						Timestamp:   proto.Int64(time.Now().UnixNano()),
					},
				}

				msgs <- envelope
			}

			go lc.HandleMessages(msgs)
			Eventually(func() bool { return checkMessageBody(port, "?report", "total: 10") }).Should(BeTrue())
			lc.Stop()
		})

		It("doesn't process events that aren't logMessages", func() {
			var (
				mu       sync.Mutex
				request  *http.Request
				statuses = make(chan int, 2)
			)
			statuses <- http.StatusOK
			statuses <- http.StatusCreated

			handler := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(<-statuses)
				mu.Lock()
				defer mu.Unlock()
				request = r
			}
			testServer := httptest.NewServer(http.HandlerFunc(handler))
			defer func() {
				Eventually(func() string {
					mu.Lock()
					defer mu.Unlock()
					if request == nil {
						return ""
					}
					return request.URL.Path
				}).Should(Equal("/report"))
				testServer.Close()
			}()

			port := testPort()

			cc := newMockCC()
			uaa := newMockUAA()
			cfg := conf(port, testServer)

			lc := logcounter.New(uaa, cc, cfg)
			go func() {
				defer GinkgoRecover()
				err := lc.Start()
				Expect(err).ToNot(HaveOccurred())
			}()

			Eventually(func() bool { return checkEndpoint(port, "?report", http.StatusOK) }).Should(BeTrue())

			msgs := make(chan *events.Envelope, 1)

			counterEnvelope := &events.Envelope{
				Origin:    proto.String("testOrigin"),
				EventType: events.Envelope_CounterEvent.Enum(),
				LogMessage: &events.LogMessage{
					Message:     []byte("testPrefix guid: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx msg: 0"),
					MessageType: events.LogMessage_OUT.Enum(),
					Timestamp:   proto.Int64(time.Now().UnixNano()),
				},
			}
			msgs <- counterEnvelope

			go lc.HandleMessages(msgs)
			Eventually(func() bool { return checkMessageBody(port, "?report", "No messages received") }).Should(BeTrue())
			lc.Stop()
		})

		It("correctly counts the number of 1008 and other errors", func() {
			var (
				mu       sync.Mutex
				request  *http.Request
				statuses = make(chan int, 2)
			)
			statuses <- http.StatusOK
			statuses <- http.StatusCreated

			handler := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(<-statuses)
				mu.Lock()
				defer mu.Unlock()
				request = r
			}
			testServer := httptest.NewServer(http.HandlerFunc(handler))
			defer func() {
				Eventually(func() string {
					mu.Lock()
					defer mu.Unlock()
					if request == nil {
						return ""
					}
					return request.URL.Path
				}).Should(Equal("/report"))
				testServer.Close()
			}()

			port := testPort()

			cc := newMockCC()
			uaa := newMockUAA()
			cfg := conf(port, testServer)

			lc := logcounter.New(uaa, cc, cfg)
			go func() {
				defer GinkgoRecover()
				err := lc.Start()
				Expect(err).ToNot(HaveOccurred())
			}()

			Eventually(func() bool { return checkEndpoint(port, "?report", http.StatusOK) }).Should(BeTrue())

			devNull, err := os.Open("/dev/null")
			Expect(err).ToNot(HaveOccurred())
			oldStderr := os.Stderr
			os.Stderr = devNull
			defer func() {
				os.Stderr = oldStderr
			}()
			terminate := make(chan os.Signal, 1)
			errs := make(chan error, 11)
			testErr := errors.New("Some Error")
			testWebsocketErr := errors.New("websocket: close 1008 Client did not respond to ping before keep-alive timeout expired.")

			closer := newMockCloser()
			close(closer.CloseOutput.Ret0)

			for i := 0; i < 2; i++ {
				errs <- testErr
				lc.HandleErrors(errs, terminate, closer)
			}

			for i := 0; i < 11; i++ {
				errs <- testWebsocketErr
				lc.HandleErrors(errs, terminate, closer)
			}

			Eventually(func() bool { return checkMessageBody(port, "?report", "1008 errors: 11") }).Should(BeTrue())
			Eventually(func() bool { return checkMessageBody(port, "?report", "Other errors: 2") }).Should(BeTrue())
			lc.Stop()
		})

		It("posts the report to logfin with the correct data", func() {
			var (
				mu       sync.Mutex
				request  *http.Request
				body     []byte
				statuses = make(chan int, 2)
			)
			statuses <- http.StatusOK
			statuses <- http.StatusCreated

			handler := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(<-statuses)
				mu.Lock()
				defer mu.Unlock()
				request = r
				body, _ = ioutil.ReadAll(r.Body)
			}
			testServer := httptest.NewServer(http.HandlerFunc(handler))
			defer func() {
				Eventually(func() string {
					mu.Lock()
					defer mu.Unlock()
					if request == nil {
						return ""
					}
					return request.URL.Path
				}).Should(Equal("/report"))
				testServer.Close()
			}()

			port := testPort()

			cc := newMockCC()
			uaa := newMockUAA()
			cfg := conf(port, testServer)

			lc := logcounter.New(uaa, cc, cfg)
			go func() {
				defer GinkgoRecover()
				err := lc.Start()
				Expect(err).ToNot(HaveOccurred())
			}()

			Eventually(func() bool { return checkEndpoint(port, "?report", http.StatusOK) }).Should(BeTrue())

			By("Sending a message")
			msgs := make(chan *events.Envelope, 1)
			envelope := &events.Envelope{
				Origin:    proto.String("testOrigin"),
				EventType: events.Envelope_LogMessage.Enum(),
				LogMessage: &events.LogMessage{
					Message:     []byte(fmt.Sprintf("testPrefix guid: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx msg: 0")),
					MessageType: events.LogMessage_OUT.Enum(),
					Timestamp:   proto.Int64(time.Now().UnixNano()),
				},
			}
			msgs <- envelope
			go lc.HandleMessages(msgs)
			Eventually(func() bool { return checkMessageBody(port, "?report", "total: 1") }).Should(BeTrue())

			devNull, err := os.Open("/dev/null")
			Expect(err).ToNot(HaveOccurred())
			oldStderr := os.Stderr
			os.Stderr = devNull
			defer func() {
				os.Stderr = oldStderr
			}()
			terminate := make(chan os.Signal, 1)
			errs := make(chan error, 2)

			By("Sending some errors")
			closer := newMockCloser()
			close(closer.CloseOutput.Ret0)

			testErr := errors.New("Some Error")
			errs <- testErr
			lc.HandleErrors(errs, terminate, closer)

			testWebsocketErr := errors.New("websocket: close 1008 Client did not respond to ping before keep-alive timeout expired.")
			errs <- testWebsocketErr
			lc.HandleErrors(errs, terminate, closer)

			statuses <- http.StatusCreated
			lc.SendReport()

			expected := map[string]map[string]interface{}{
				"errors": {
					"1008":  float64(1),
					"other": float64(1),
				},
				"messages": {
					"xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx": map[string]interface{}{
						"app":   "",
						"total": float64(1),
						"max":   float64(1),
					},
				},
			}

			var jsBody map[string]map[string]interface{}

			mu.Lock()
			defer mu.Unlock()
			Expect(request.Method).To(Equal("POST"))
			Expect(request.URL.Path).To(Equal("/report"))
			Expect(json.Unmarshal(body, &jsBody)).To(Succeed())
			Expect(jsBody).To(Equal(expected))
			lc.Stop()
		})

		It("requests the status after the runtime duration until the status is OK", func() {
			statuses := make(chan int, 1)
			requests := make(chan *http.Request)
			handler := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(<-statuses)
				requests <- r
			}
			testServer := httptest.NewServer(http.HandlerFunc(handler))
			defer testServer.Close()

			port := testPort()

			cc := newMockCC()
			uaa := newMockUAA()
			cfg := conf(port, testServer)
			cfg.Runtime = 500 * time.Millisecond

			lc := logcounter.New(uaa, cc, cfg)
			go func() {
				defer GinkgoRecover()
				err := lc.Start()
				Expect(err).ToNot(HaveOccurred())
			}()

			Eventually(func() bool { return checkEndpoint(port, "?report", http.StatusOK) }).Should(BeTrue())
			statuses <- http.StatusPreconditionFailed
			Consistently(requests, 400*time.Millisecond).ShouldNot(Receive())
			Eventually(requests).Should(Receive())
			statuses <- http.StatusOK
			Eventually(requests).Should(Receive())

			statuses <- http.StatusCreated
			var reportReq *http.Request
			Eventually(requests).Should(Receive(&reportReq))
			Expect(reportReq.URL.Path).To(Equal("/report"))
			Expect(reportReq.Method).To(Equal("POST"))

			lc.Stop()
		})
	})
})

func testPort() string {
	add, _ := net.ResolveTCPAddr("tcp", ":0")
	l, _ := net.ListenTCP("tcp", add)
	defer l.Close()
	port := strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
	return port
}

func checkEndpoint(port, endpoint string, status int) bool {
	resp, _ := http.Get("http://localhost:" + port + "/" + endpoint)
	if resp != nil {
		return resp.StatusCode == status
	}

	return false
}

func checkMessageBody(port, endpoint, expected string) bool {
	resp, _ := http.Get("http://localhost:" + port + "/" + endpoint)
	if resp != nil {
		body, _ := ioutil.ReadAll(resp.Body)
		if strings.Contains(string(body), expected) {
			return true
		}
	}

	return false
}

func conf(port string, testServer *httptest.Server) *config.Config {
	return &config.Config{
		ApiURL:         "api.example.com",
		DopplerURL:     "doppler.example.com",
		UaaURL:         "uaa.example.com",
		ClientID:       "testID",
		ClientSecret:   "clientSecret",
		Username:       "testUserName",
		Password:       "testPassword",
		MessagePrefix:  "testPrefix",
		SubscriptionID: "testSubID",
		Port:           port,
		LogfinURL:      testServer.URL,
	}
}
