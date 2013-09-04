package loggregatorrouter

import (
	"errors"
	testhelpers "github.com/cloudfoundry/loggregatorlib/lib_testhelpers"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

const (
	redirector_host = "0.0.0.0"
	redirector_port = "4443"
)

func TestThatItRedirects(t *testing.T) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) (err error) { return errors.New("Got redirect") },
	}
	loggregatorServers := []string{"10.10.10.10:9991", "10.20.30.40:9992"}
	hasher := NewHasher(loggregatorServers)
	r := NewRedirector(redirector_host+":"+redirector_port, hasher, testhelpers.Logger())
	go r.Start()

	// appId that hashes to first loggregatorServer entry
	appId := "appId"
	endpoint := "http://localhost:4443/tail/?app=" + appId
	expectedUrl := "wss://10-10-10-10-9991-localhost:4443/tail/?app=" + appId

	req, err := http.NewRequest("GET", endpoint, nil)
	assert.NoError(t, err)
	resp, err := client.Do(req)
	assert.Equal(t, resp.StatusCode, 302)
	assert.Equal(t, resp.Header.Get("Location"), expectedUrl)

	// appId that hashes to second loggregatorServer entry
	appId = "c53734e6-a5ef-45dd-b62f-827158356fa5"
	endpoint = "http://localhost:4443/tail/?app=" + appId
	expectedUrl = "wss://10-20-30-40-9992-localhost:4443/tail/?app=" + appId

	req, err = http.NewRequest("GET", endpoint, nil)
	assert.NoError(t, err)
	resp, err = client.Do(req)
	assert.Equal(t, resp.StatusCode, 302)
	assert.Equal(t, resp.Header.Get("Location"), expectedUrl)
}

func TestThatItGeneratesRedirectUrlWithoutProtoHeaderWithSSLHostPort(t *testing.T) {
	loggregatorServers := []string{"10.10.10.10:9991", "10.20.30.40:9992"}
	hasher := NewHasher(loggregatorServers)
	r := NewRedirector(redirector_host+":"+redirector_port, hasher, testhelpers.Logger())

	fakeReq0, _ := http.NewRequest("GET", "https://localhost:4443/tail/?app=appId", nil)
	expectedUrl0 := "wss://10-10-10-10-9991-localhost:4443/tail/?app=appId"
	redirectUrl0 := r.generateRedirectUrl(fakeReq0)

	assert.Equal(t, redirectUrl0, expectedUrl0)
}

func TestThatItGeneratesRedirectUrlWithoutProtoHeaderWithNonSSLHostPort(t *testing.T) {
	loggregatorServers := []string{"10.10.10.10:9991", "10.20.30.40:9992"}
	hasher := NewHasher(loggregatorServers)
	r := NewRedirector(redirector_host+":"+redirector_port, hasher, testhelpers.Logger())

	fakeReq0, _ := http.NewRequest("GET", "http://localhost:1234/tail/?app=appId", nil)
	expectedUrl0 := "ws://10-10-10-10-9991-localhost:1234/tail/?app=appId"
	redirectUrl0 := r.generateRedirectUrl(fakeReq0)

	assert.Equal(t, redirectUrl0, expectedUrl0)
}

func TestThatItGeneratesRedirectUrlWithoutProtoHeaderWithoutHostPort(t *testing.T) {
	loggregatorServers := []string{"10.10.10.10:9991", "10.20.30.40:9992"}
	hasher := NewHasher(loggregatorServers)
	r := NewRedirector(redirector_host+":"+redirector_port, hasher, testhelpers.Logger())

	fakeReq1, _ := http.NewRequest("GET", "http://localhost/tail/?app=appId", nil)
	expectedUrl1 := "ws://10-10-10-10-9991-localhost/tail/?app=appId"
	redirectUrl1 := r.generateRedirectUrl(fakeReq1)

	assert.Equal(t, redirectUrl1, expectedUrl1)
}

func TestThatItGeneratesRedirectUrlWithProtoHeaderWithHostPort(t *testing.T) {
	loggregatorServers := []string{"10.10.10.10:9991", "10.20.30.40:9992"}
	hasher := NewHasher(loggregatorServers)

	r := NewRedirector(redirector_host+":"+redirector_port, hasher, testhelpers.Logger())

	fakeReq0, _ := http.NewRequest("GET", "https://localhost:443/tail/?app=appId", nil)
	fakeReq0.Header.Set("X-Forwarded-Proto", "foo")

	expectedUrl0 := "foo://10-10-10-10-9991-localhost:443/tail/?app=appId"
	redirectUrl0 := r.generateRedirectUrl(fakeReq0)

	assert.Equal(t, redirectUrl0, expectedUrl0)
}

func TestThatItGeneratesRedirectUrlWithProtoHeaderWithoutHostPort(t *testing.T) {
	loggregatorServers := []string{"10.10.10.10:9991", "10.20.30.40:9992"}
	hasher := NewHasher(loggregatorServers)

	r := NewRedirector(redirector_host+":"+redirector_port, hasher, testhelpers.Logger())

	fakeReq0, _ := http.NewRequest("GET", "https://localhost/tail/?app=appId", nil)
	fakeReq0.Header.Set("X-Forwarded-Proto", "foo")

	expectedUrl0 := "foo://10-10-10-10-9991-localhost/tail/?app=appId"
	redirectUrl0 := r.generateRedirectUrl(fakeReq0)

	assert.Equal(t, redirectUrl0, expectedUrl0)
}
