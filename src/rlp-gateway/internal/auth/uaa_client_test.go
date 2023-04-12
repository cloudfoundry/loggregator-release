package auth_test

import (
	"io"
	"log"
	"sync"

	"code.cloudfoundry.org/loggregator-release/src/internal/testhelper"

	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	"code.cloudfoundry.org/loggregator-release/src/rlp-gateway/internal/auth"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("UAAClient", func() {
	var (
		client     *auth.UAAClient
		httpClient *spyHTTPClient
		metrics    *testhelper.SpyMetricClient
	)

	BeforeEach(func() {
		httpClient = newSpyHTTPClient()
		metrics = testhelper.NewMetricClient()
		client = auth.NewUAAClient(
			"https://uaa.com",
			"some-client",
			"some-client-secret",
			httpClient,
			metrics,
			log.New(io.Discard, "", 0),
		)
	})

	It("returns IsAdmin when scopes include doppler.firehose with correct clientID and UserID", func() {
		data, err := json.Marshal(map[string]interface{}{
			"scope":     []string{"doppler.firehose"},
			"user_id":   "some-user",
			"client_id": "some-client",
		})
		Expect(err).ToNot(HaveOccurred())

		httpClient.resps = []response{{
			body:   data,
			status: http.StatusOK,
		}}

		c, err := client.Read("valid-token")
		Expect(err).ToNot(HaveOccurred())
		Expect(c.IsAdmin).To(BeTrue())
		Expect(c.UserID).To(Equal("some-user"))
		Expect(c.ClientID).To(Equal("some-client"))
	})

	It("returns IsAdmin when scopes include logs.admin with correct clientID and UserID", func() {
		data, err := json.Marshal(map[string]interface{}{
			"scope":     []string{"logs.admin"},
			"user_id":   "some-user",
			"client_id": "some-client",
		})
		Expect(err).ToNot(HaveOccurred())
		httpClient.resps = []response{{
			body:   data,
			status: http.StatusOK,
		}}

		c, err := client.Read("valid-token")
		Expect(err).ToNot(HaveOccurred())
		Expect(c.IsAdmin).To(BeTrue())
		Expect(c.UserID).To(Equal("some-user"))
		Expect(c.ClientID).To(Equal("some-client"))
	})

	It("calls UAA correctly", func() {
		token := "my-token"
		_, err := client.Read(token)
		Expect(err).To(Equal(io.EOF))

		r := httpClient.requests[0]

		Expect(r.Method).To(Equal(http.MethodPost))
		Expect(r.Header.Get("Content-Type")).To(
			Equal("application/x-www-form-urlencoded"),
		)
		Expect(r.URL.String()).To(Equal("https://uaa.com/check_token"))

		client, clientSecret, ok := r.BasicAuth()
		Expect(ok).To(BeTrue())
		Expect(client).To(Equal("some-client"))
		Expect(clientSecret).To(Equal("some-client-secret"))

		reqBytes, err := io.ReadAll(r.Body)
		Expect(err).ToNot(HaveOccurred())
		urlValues, err := url.ParseQuery(string(reqBytes))
		Expect(err).ToNot(HaveOccurred())
		Expect(urlValues.Get("token")).To(Equal(token))
	})

	It("sets the last request latency metric", func() {
		_, err := client.Read("my-token")
		Expect(err).To(Equal(io.EOF))

		Expect(metrics.GetMetric("LastUAALatency", nil).Value()).ToNot(BeZero())
	})

	It("returns error when token is blank", func() {
		_, err := client.Read("")
		Expect(err).To(HaveOccurred())
	})

	It("returns false when scopes don't include doppler.firehose or logs.admin", func() {
		data, err := json.Marshal(map[string]interface{}{
			"scope":     []string{"some-scope"},
			"user_id":   "some-user",
			"client_id": "some-client",
		})
		Expect(err).ToNot(HaveOccurred())
		httpClient.resps = []response{{
			body:   data,
			status: http.StatusOK,
		}}

		c, err := client.Read("invalid-token")
		Expect(err).ToNot(HaveOccurred())

		Expect(c.IsAdmin).To(BeFalse())
	})

	It("returns false when the request fails", func() {
		httpClient.resps = []response{{
			err: errors.New("some-err"),
		}}

		_, err := client.Read("valid-token")
		Expect(err).To(HaveOccurred())
	})

	It("returns false when the response from the UAA is invalid", func() {
		httpClient.resps = []response{{
			body:   []byte("garbage"),
			status: http.StatusOK,
		}}

		_, err := client.Read("valid-token")
		Expect(err).To(HaveOccurred())
	})
})

type spyHTTPClient struct {
	mu       sync.Mutex
	requests []*http.Request
	resps    []response
}

type response struct {
	status int
	err    error
	body   []byte
}

func newSpyHTTPClient() *spyHTTPClient {
	return &spyHTTPClient{}
}

func (s *spyHTTPClient) Do(r *http.Request) (*http.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.requests = append(s.requests, r)

	if len(s.resps) == 0 {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(nil)),
		}, nil
	}

	result := s.resps[0]
	s.resps = s.resps[1:]

	resp := http.Response{
		StatusCode: result.status,
		Body:       io.NopCloser(bytes.NewReader(result.body)),
	}

	if result.err != nil {
		return nil, result.err
	}

	return &resp, nil
}
