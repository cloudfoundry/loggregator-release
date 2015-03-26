package main_test

import (
	syslog_drain_binder "syslog_drain_binder"

	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"syslog_drain_binder/shared_types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CloudControllerPoller", func() {
	var _ = Describe("GetSyslogDrainURLs", func() {
		var (
			testServer          *httptest.Server
			fakeCloudController fakeCC
			addr                string
		)

		BeforeEach(func() {
			fakeCloudController = fakeCC{}

			testServer = httptest.NewServer(http.HandlerFunc(fakeCloudController.ServeHTTP))
			addr = "http://" + testServer.Listener.Addr().String()
		})

		AfterEach(func() {
			testServer.Close()
		})

		It("connects to the correct endpoint with basic authentication and the expected parameters", func() {
			syslog_drain_binder.Poll(addr, "user", "pass", 2, false)
			Expect(fakeCloudController.servedRoute).To(Equal("/v2/syslog_drain_urls"))
			Expect(fakeCloudController.username).To(Equal("user"))
			Expect(fakeCloudController.password).To(Equal("pass"))

			Expect(fakeCloudController.queryParams).To(HaveKeyWithValue("batch_size", []string{"2"}))
		})

		It("processes all pages into a single result with batch_size 2", func() {
			drainUrls, err := syslog_drain_binder.Poll(addr, "user", "pass", 2, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeCloudController.requestCount).To(Equal(6))

			for _, entry := range appDrains {
				Expect(drainUrls).To(HaveKeyWithValue(entry.appId, entry.urls))
			}
		})

		It("processes all pages into a single result with batch_size 3", func() {
			drainUrls, err := syslog_drain_binder.Poll(addr, "user", "pass", 3, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeCloudController.requestCount).To(Equal(5))

			for _, entry := range appDrains {
				Expect(drainUrls).To(HaveKeyWithValue(entry.appId, entry.urls))
			}
		})

		Context("when CC becomes unreachable before finishing", func() {
			BeforeEach(func() {
				fakeCloudController.failOn = 4
			})

			It("returns as much data as it has, and an error", func() {
				drainUrls, err := syslog_drain_binder.Poll(addr, "user", "pass", 2, false)
				Expect(err).To(HaveOccurred())

				Expect(fakeCloudController.requestCount).To(Equal(4))

				for i := 0; i < 8; i++ {
					entry := appDrains[i]
					Expect(drainUrls).To(HaveKeyWithValue(entry.appId, entry.urls))
				}

				for i := 8; i < 10; i++ {
					entry := appDrains[i]
					Expect(drainUrls).NotTo(HaveKeyWithValue(entry.appId, entry.urls))
				}
			})
		})

		Context("when connecting to a secure server with a self-signed certificate", func() {
			var secureTestServer *httptest.Server

			BeforeEach(func() {
				secureTestServer = httptest.NewTLSServer(http.HandlerFunc(fakeCloudController.ServeHTTP))

				addr = "https://" + secureTestServer.Listener.Addr().String()
			})

			It("fails to connect if skipCertVerify is false", func() {
				secureTestServer = httptest.NewTLSServer(http.HandlerFunc(fakeCloudController.ServeHTTP))

				addr = "https://" + secureTestServer.Listener.Addr().String()
				_, err := syslog_drain_binder.Poll(addr, "user", "pass", 2, false)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("certificate signed by unknown authority"))
			})

			It("successfully connects if skipCertVerify is true", func() {
				secureTestServer := httptest.NewTLSServer(http.HandlerFunc(fakeCloudController.ServeHTTP))

				addr = "https://" + secureTestServer.Listener.Addr().String()
				_, err := syslog_drain_binder.Poll(addr, "user", "pass", 2, true)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})

type appEntry struct {
	appId shared_types.AppId
	urls  []shared_types.DrainURL
}

var appDrains = []appEntry{
	{appId: "app0", urls: []shared_types.DrainURL{"urlA"}},
	{appId: "app1", urls: []shared_types.DrainURL{"urlB"}},
	{appId: "app2", urls: []shared_types.DrainURL{"urlA", "urlC"}},
	{appId: "app3", urls: []shared_types.DrainURL{"urlA", "urlD", "urlE"}},
	{appId: "app4", urls: []shared_types.DrainURL{"urlA"}},
	{appId: "app5", urls: []shared_types.DrainURL{"urlA"}},
	{appId: "app6", urls: []shared_types.DrainURL{"urlA"}},
	{appId: "app7", urls: []shared_types.DrainURL{"urlA"}},
	{appId: "app8", urls: []shared_types.DrainURL{"urlA"}},
	{appId: "app9", urls: []shared_types.DrainURL{"urlA"}},
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
	if start >= 10 {
		r = jsonResponse{
			Results: make(map[shared_types.AppId][]shared_types.DrainURL),
			NextId:  nil,
		}
	} else {
		r = jsonResponse{
			Results: make(map[shared_types.AppId][]shared_types.DrainURL),
			NextId:  &end,
		}

		for i := start; i < end && i < 10; i++ {
			r.Results[appDrains[i].appId] = appDrains[i].urls
		}
	}

	b, _ := json.Marshal(r)
	return b
}

type jsonResponse struct {
	Results map[shared_types.AppId][]shared_types.DrainURL `json:"results"`
	NextId  *int                                           `json:"next_id"`
}
