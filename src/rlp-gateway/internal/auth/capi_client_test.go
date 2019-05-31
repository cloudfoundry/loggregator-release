package auth_test

import (
	"fmt"
	"io/ioutil"
	"log"
	"sync"

	"errors"
	"net/http"

	"code.cloudfoundry.org/loggregator/rlp-gateway/internal/auth"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CAPIClient", func() {
	var (
		capiClient *spyHTTPClient
		client     *auth.CAPIClient
		metrics    *spyMetrics
	)

	BeforeEach(func() {
		capiClient = newSpyHTTPClient()
		metrics = newSpyMetrics()
		client = auth.NewCAPIClient(
			"https://capi.com",
			"http://external.capi.com",
			capiClient,
			metrics,
			log.New(ioutil.Discard, "", 0),
		)
	})

	Describe("IsAuthorized", func() {
		It("hits CAPI correctly", func() {
			capiClient.resps = []response{
				{status: http.StatusNotFound},
			}

			client.IsAuthorized("some-id", "some-token")
			r := capiClient.requests[0]

			Expect(r.Method).To(Equal(http.MethodGet))
			Expect(r.URL.String()).To(Equal("https://capi.com/internal/v4/log_access/some-id"))
			Expect(r.Header.Get("Authorization")).To(Equal("some-token"))

			r = capiClient.requests[1]

			Expect(r.Method).To(Equal(http.MethodGet))
			Expect(r.URL.String()).To(Equal("http://external.capi.com/v2/service_instances/some-id"))
			Expect(r.Header.Get("Authorization")).To(Equal("some-token"))
		})

		It("returns true for authorized token for an app", func() {
			capiClient.resps = []response{{status: http.StatusOK}}
			Expect(client.IsAuthorized("some-id", "some-token")).To(BeTrue())
		})

		It("returns true for authorized token for a service instance", func() {
			capiClient.resps = []response{
				{status: http.StatusNotFound},
				{status: http.StatusOK},
			}
			Expect(client.IsAuthorized("some-id", "some-token")).To(BeTrue())
		})

		It("returns false when CAPI returns non 200 for app and service instance", func() {
			capiClient.resps = []response{
				{status: http.StatusNotFound},
				{status: http.StatusNotFound},
			}
			Expect(client.IsAuthorized("some-id", "some-token")).To(BeFalse())
		})

		It("returns false when CAPI request fails", func() {
			capiClient.resps = []response{{err: errors.New("intentional error")}}
			Expect(client.IsAuthorized("some-id", "some-token")).To(BeFalse())
		})

		It("stores the latency", func() {
			capiClient.resps = []response{
				{status: http.StatusNotFound},
			}
			client.IsAuthorized("source-id", "my-token")

			Expect(metrics.m["LastCAPIV4LogAccessLatency"]).ToNot(BeZero())
			Expect(metrics.m["LastCAPIV2ServiceInstancesLatency"]).ToNot(BeZero())
		})

		It("is goroutine safe", func() {
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				for i := 0; i < 1000; i++ {
					client.IsAuthorized(fmt.Sprintf("some-id-%d", i), "some-token")
				}

				wg.Done()
			}()

			for i := 0; i < 1000; i++ {
				client.IsAuthorized(fmt.Sprintf("some-id-%d", i), "some-token")
			}
			wg.Wait()
		})
	})

	Describe("AvailableSourceIDs", func() {
		It("hits CAPI correctly", func() {
			client.AvailableSourceIDs("some-token")
			Expect(capiClient.requests).To(HaveLen(2))

			appsReq := capiClient.requests[0]
			Expect(appsReq.Method).To(Equal(http.MethodGet))
			Expect(appsReq.URL.String()).To(Equal("http://external.capi.com/v3/apps"))
			Expect(appsReq.Header.Get("Authorization")).To(Equal("some-token"))

			servicesReq := capiClient.requests[1]
			Expect(servicesReq.Method).To(Equal(http.MethodGet))
			Expect(servicesReq.URL.String()).To(Equal("http://external.capi.com/v2/service_instances"))
			Expect(servicesReq.Header.Get("Authorization")).To(Equal("some-token"))
		})

		It("returns the available app and service instance IDs", func() {
			capiClient.resps = []response{
				{status: http.StatusOK, body: []byte(`{"resources": [{"guid": "app-0"}, {"guid": "app-1"}]}`)},
				{status: http.StatusOK, body: []byte(`{"resources": [{"metadata":{"guid": "service-2"}}, {"metadata":{"guid": "service-3"}}]}`)},
			}
			sourceIDs := client.AvailableSourceIDs("some-token")
			Expect(sourceIDs).To(ConsistOf("app-0", "app-1", "service-2", "service-3"))
		})

		It("returns empty slice when CAPI apps request returns non 200", func() {
			capiClient.resps = []response{
				{status: http.StatusNotFound},
				{status: http.StatusOK, body: []byte(`{"resources": [{"metadata":{"guid": "service-2"}}, {"metadata":{"guid": "service-3"}}]}`)},
			}
			sourceIDs := client.AvailableSourceIDs("some-token")
			Expect(sourceIDs).To(BeEmpty())
		})

		It("returns empty slice when CAPI apps request fails", func() {
			capiClient.resps = []response{
				{err: errors.New("intentional error")},
				{status: http.StatusOK, body: []byte(`{"resources": [{"metadata":{"guid": "service-2"}}, {"metadata":{"guid": "service-3"}}]}`)},
			}
			sourceIDs := client.AvailableSourceIDs("some-token")
			Expect(sourceIDs).To(BeEmpty())
		})

		It("returns empty slice when CAPI service_instances request returns non 200", func() {
			capiClient.resps = []response{
				{status: http.StatusOK, body: []byte(`{"resources": [{"guid": "app-0"}, {"guid": "app-1"}]}`)},
				{status: http.StatusNotFound},
			}
			sourceIDs := client.AvailableSourceIDs("some-token")
			Expect(sourceIDs).To(BeEmpty())
		})

		It("returns empty slice when CAPI service_instances request fails", func() {
			capiClient.resps = []response{
				{status: http.StatusOK, body: []byte(`{"resources": [{"guid": "app-0"}, {"guid": "app-1"}]}`)},
				{err: errors.New("intentional error")},
			}
			sourceIDs := client.AvailableSourceIDs("some-token")
			Expect(sourceIDs).To(BeEmpty())
		})

		It("stores the latency", func() {
			client.AvailableSourceIDs("my-token")

			Expect(metrics.m["LastCAPIV3AppsLatency"]).ToNot(BeZero())
			Expect(metrics.m["LastCAPIV2ListServiceInstancesLatency"]).ToNot(BeZero())
		})

		It("is goroutine safe", func() {
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				for i := 0; i < 1000; i++ {
					client.AvailableSourceIDs("some-token")
				}

				wg.Done()
			}()

			for i := 0; i < 1000; i++ {
				client.AvailableSourceIDs("some-token")
			}
			wg.Wait()
		})
	})
})
