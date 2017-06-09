package main_test

import (
	"crypto/tls"
	"net"
	"time"

	syslog_drain_binder "code.cloudfoundry.org/loggregator/syslog_drain_binder"
	"code.cloudfoundry.org/loggregator/syslog_drain_binder/fake_cc"
	"code.cloudfoundry.org/loggregator/syslog_drain_binder/shared_types"

	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CloudControllerPoller", func() {
	var _ = Describe("GetSyslogDrainURLs", func() {
		var (
			testServer          *httptest.Server
			fakeCloudController *fake_cc.FakeCC
			baseURL             string
			tlsConfig           *tls.Config
		)

		BeforeEach(func() {
			fakeCloudController = fake_cc.NewFakeCC([]fake_cc.AppEntry{
				{
					AppId: "app0",
					SyslogBinding: fake_cc.SysLogBinding{
						Hostname:  "org.space.app.1",
						DrainURLs: []string{"urlA"},
					},
				},
				{
					AppId: "app1",
					SyslogBinding: fake_cc.SysLogBinding{
						Hostname:  "org.space.app.2",
						DrainURLs: []string{"urlB"},
					},
				},
				{
					AppId: "app2",
					SyslogBinding: fake_cc.SysLogBinding{
						Hostname:  "org.space.app.3",
						DrainURLs: []string{"urlA", "urlC"},
					},
				},
			})

			tlsConfig = &tls.Config{
				InsecureSkipVerify: true,
			}
			testServer = httptest.NewTLSServer(
				http.HandlerFunc(fakeCloudController.ServeHTTP),
			)
			baseURL = testServer.URL
		})

		AfterEach(func() {
			testServer.Close()
		})

		It("connects to the correct endpoint with basic authentication and the expected parameters", func() {
			syslog_drain_binder.Poll(baseURL, 2, tlsConfig)

			Expect(fakeCloudController.ServedRoute).To(Equal("/internal/v4/syslog_drain_urls"))
			Expect(fakeCloudController.QueryParams).To(HaveKeyWithValue("batch_size", []string{"2"}))
		})

		It("returns sys log drain bindings for all apps", func() {
			drainUrls, err := syslog_drain_binder.Poll(baseURL, 3, tlsConfig)
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
			_, err := syslog_drain_binder.Poll(baseURL, 2, tlsConfig)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeCloudController.RequestCount).To(Equal(3))
		})

		Context("when CC becomes unreachable before finishing", func() {
			It("returns as much data as it has, and an error", func() {
				fakeCloudController.FailOn = 2

				_, err := syslog_drain_binder.Poll(baseURL, 2, tlsConfig)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("with the cloud controller not responding", func() {
			It("times out by default", func() {
				serverNotResponding, err := net.Listen("tcp", ":0")
				Expect(err).ToNot(HaveOccurred())
				orig := syslog_drain_binder.DefaultTimeout
				syslog_drain_binder.DefaultTimeout = 10 * time.Millisecond
				defer func() { syslog_drain_binder.DefaultTimeout = orig }()
				baseURL = "https://" + serverNotResponding.Addr().String()

				errs := make(chan error)
				go func() {
					_, err := syslog_drain_binder.Poll(baseURL, 2, tlsConfig)
					errs <- err
				}()

				Eventually(errs).Should(Receive())
			})
		})
	})
})
