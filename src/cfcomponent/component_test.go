package cfcomponent

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"testing"
)

type FakeHealthMonitor struct {
	healthy bool
}

func (hm FakeHealthMonitor) Ok() bool {
	return hm.healthy
}

var healthMonitor FakeHealthMonitor

func init() {
	healthMonitor = FakeHealthMonitor{}
	component := &Component{
		HealthMonitor:     &healthMonitor,
		StatusPort:        7876,
		StatusCredentials: []string{"user", "pass"},
	}

	component.StartHealthz()
}

func TestGoodHealthzEndpoint(t *testing.T) {
	healthMonitor.healthy = true
	req, err := http.NewRequest("GET", "http://localhost:7876/healtz", nil)
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)

	assert.Equal(t, resp.StatusCode, 200)
	assert.Equal(t, resp.Header.Get("Content-Type"), "text/plain")
	body, err := ioutil.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, string(body), "ok")

}

func TestBadHealthzEndpoint(t *testing.T) {
	healthMonitor.healthy = false
	req, err := http.NewRequest("GET", "http://localhost:7876/healtz", nil)
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)

	assert.Equal(t, resp.StatusCode, 200)
	body, err := ioutil.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, string(body), "bad")
}
