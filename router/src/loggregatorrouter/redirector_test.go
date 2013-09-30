package loggregatorrouter

import (
	"errors"
	testhelpers "github.com/cloudfoundry/loggregatorlib/lib_testhelpers"
	"github.com/stretchr/testify/assert"
	"loggregatorrouter/hasher"
	"net/http"
	"testing"
)

var redirectTests = []struct {
	endpoint       string
	expectedUrl    string
	forwardedProto string
}{
	{
		"http://localhost:4443/tail/?app=appId",
		"ws://10-10-10-10-9991-localhost:4443/tail/?app=appId",
		"http",
	},
	{
		"http://localhost:4443/tail/?app=c53734e6-a5ef-45dd-b62f-827158356fa5",
		"wss://10-20-30-40-9992-localhost:4443/tail/?app=c53734e6-a5ef-45dd-b62f-827158356fa5",
		"https",
	},
	{
		"http://localhost:4443/dump/?app=appId",
		"ws://10-10-10-10-9991-localhost:4443/dump/?app=appId",
		"http",
	},
	{
		"http://localhost:4443/dump/?app=c53734e6-a5ef-45dd-b62f-827158356fa5",
		"wss://10-20-30-40-9992-localhost:4443/dump/?app=c53734e6-a5ef-45dd-b62f-827158356fa5",
		"https",
	},
}

func TestRedirects(t *testing.T) {
	loggregatorServers := []string{"10.10.10.10:9991", "10.20.30.40:9992"}
	hasher := hasher.NewHasher(loggregatorServers)
	r := NewRedirector("0.0.0.0:4443", hasher, testhelpers.Logger())
	go r.Start()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) (err error) { return errors.New("Got redirect") },
	}

	for _, test := range redirectTests {
		req, err := http.NewRequest("GET", test.endpoint, nil)
		req.Header.Set("X-Forwarded-Proto", test.forwardedProto)
		assert.NoError(t, err)
		resp, err := client.Do(req)
		assert.Equal(t, resp.StatusCode, 302)
		assert.Equal(t, resp.Header.Get("Location"), test.expectedUrl)
	}
}
