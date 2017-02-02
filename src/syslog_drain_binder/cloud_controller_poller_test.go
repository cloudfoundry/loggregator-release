package main_test

import (
	"crypto/tls"
	"net"
	syslog_drain_binder "syslog_drain_binder"
	"syslog_drain_binder/shared_types"
	"time"

	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CloudControllerPoller", func() {
	var _ = Describe("GetSyslogDrainURLs", func() {
		var (
			testServer          *httptest.Server
			fakeCloudController fakeCC
			baseURL             string
		)

		BeforeEach(func() {
			fakeCloudController = fakeCC{}

			testServer = httptest.NewServer(
				http.HandlerFunc(fakeCloudController.ServeHTTP),
			)
			baseURL = testServer.URL
		})

		AfterEach(func() {
			testServer.Close()
		})

		It("connects to the correct endpoint with basic authentication and the expected parameters", func() {
			syslog_drain_binder.Poll(baseURL, "user", "pass", 2)

			Expect(fakeCloudController.servedRoute).To(Equal("/v2/syslog_drain_urls"))
			Expect(fakeCloudController.username).To(Equal("user"))
			Expect(fakeCloudController.password).To(Equal("pass"))
			Expect(fakeCloudController.queryParams).To(HaveKeyWithValue("batch_size", []string{"2"}))
		})

		It("returns sys log drain bindings for all apps", func() {
			drainUrls, err := syslog_drain_binder.Poll(baseURL, "user", "pass", 3)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(drainUrls)).To(Equal(3))

			Expect(drainUrls["app0"]).To(Equal(
				shared_types.SyslogDrainBinding{
					DrainURLs: []string{"urlA"},
					Hostname:  "org.space.app.1",
				}),
			)
			Expect(drainUrls["app1"]).To(Equal(
				shared_types.SyslogDrainBinding{
					DrainURLs: []string{"urlB"},
					Hostname:  "org.space.app.2",
				}),
			)
			Expect(drainUrls["app2"]).To(Equal(
				shared_types.SyslogDrainBinding{
					DrainURLs: []string{"urlA", "urlC"},
					Hostname:  "org.space.app.3",
				}),
			)
		})

		It("issues multiple requests to support pagination", func() {
			_, err := syslog_drain_binder.Poll(baseURL, "user", "pass", 2)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeCloudController.requestCount).To(Equal(3))
		})

		Context("when CC becomes unreachable before finishing", func() {
			It("returns as much data as it has, and an error", func() {
				fakeCloudController.failOn = 2

				_, err := syslog_drain_binder.Poll(baseURL, "user", "pass", 2)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when connecting to a secure server with a self-signed certificate", func() {
			var secureTestServer *httptest.Server

			BeforeEach(func() {
				secureTestServer = httptest.NewUnstartedServer(http.HandlerFunc(fakeCloudController.ServeHTTP))
				secureTestServer.TLS = &tls.Config{
					CipherSuites: []uint16{
						tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
						tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					},
					MinVersion: tls.VersionTLS12,
				}
				secureTestServer.StartTLS()
				baseURL = "https://" + secureTestServer.Listener.Addr().String()
			})

			AfterEach(func() {
				secureTestServer.Close()
			})

			It("fails to connect if skipCertVerify is false", func() {
				_, err := syslog_drain_binder.Poll(baseURL, "user", "pass", 2)
				Expect(err).To(HaveOccurred())
			})

			It("successfully connects if skipCertVerify is true", func() {
				_, err := syslog_drain_binder.Poll(baseURL, "user", "pass", 2, syslog_drain_binder.SkipCertVerify(true))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("with the cloud controller not responding", func() {
			It("times out by default", func() {
				serverNotResponding, err := net.Listen("tcp", ":0")
				Expect(err).ToNot(HaveOccurred())
				orig := syslog_drain_binder.DefaultTimeout
				syslog_drain_binder.DefaultTimeout = 10 * time.Millisecond
				defer func() { syslog_drain_binder.DefaultTimeout = orig }()
				baseURL = "http://" + serverNotResponding.Addr().String()

				errs := make(chan error)
				go func() {
					_, err := syslog_drain_binder.Poll(baseURL, "user", "pass", 2)
					errs <- err
				}()

				Eventually(errs).Should(Receive())
			})
		})
	})
})

type appEntry struct {
	appID         string
	syslogBinding SysLogBinding
}

var appDrains = []appEntry{
	{
		appID: "app0",
		syslogBinding: SysLogBinding{
			Hostname:  "org.space.app.1",
			DrainURLs: []string{"urlA"},
		},
	},
	{
		appID: "app1",
		syslogBinding: SysLogBinding{
			Hostname:  "org.space.app.2",
			DrainURLs: []string{"urlB"},
		},
	},
	{
		appID: "app2",
		syslogBinding: SysLogBinding{
			Hostname:  "org.space.app.3",
			DrainURLs: []string{"urlA", "urlC"},
		},
	},
}

type SysLogBinding struct {
	Hostname  string   `json:"hostname"`
	DrainURLs []string `json:"drains"`
}

type jsonResponse struct {
	Results map[string]SysLogBinding `json:"results"`
	NextId  *int                     `json:"next_id"`
}

type fakeCC struct {
	servedRoute  string
	username     string
	password     string
	queryParams  url.Values
	requestCount int
	failOn       int
}

func (fake *fakeCC) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if fake.failOn > 0 && fake.requestCount >= fake.failOn {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	fake.requestCount++
	fake.servedRoute = r.URL.Path
	fake.queryParams = r.URL.Query()

	auth := r.Header.Get("Authorization")
	parts := strings.Split(auth, " ")
	decodedBytes, _ := base64.StdEncoding.DecodeString(parts[1])
	creds := strings.Split(string(decodedBytes), ":")

	fake.username = creds[0]
	fake.password = creds[1]

	batchSize, _ := strconv.Atoi(fake.queryParams.Get("batch_size"))
	start, _ := strconv.Atoi(fake.queryParams.Get("next_id"))

	w.Write(buildResponse(start, start+batchSize))
}

func buildResponse(start int, end int) []byte {
	var r jsonResponse
	if start >= len(appDrains) {
		r = jsonResponse{
			Results: make(map[string]SysLogBinding),
			NextId:  nil,
		}
		b, _ := json.Marshal(r)
		return b
	}

	r = jsonResponse{
		Results: make(map[string]SysLogBinding),
		NextId:  &end,
	}

	for i := start; i < end && i < len(appDrains); i++ {
		r.Results[appDrains[i].appID] = appDrains[i].syslogBinding
	}

	b, _ := json.Marshal(r)
	return b
}
