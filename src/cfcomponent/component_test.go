package cfcomponent

import (
	"cfcomponent/instrumentation"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"runtime"
	"testing"
)

type GoodHealthMonitor struct{}

func (hm GoodHealthMonitor) Ok() bool {
	return true
}

type BadHealthMonitor struct{}

func (hm BadHealthMonitor) Ok() bool {
	return false
}

func TestGoodHealthzEndpoint(t *testing.T) {
	component := &Component{
		HealthMonitor:     GoodHealthMonitor{},
		StatusPort:        7876,
		Type:              "loggregator",
		StatusCredentials: []string{"user", "pass"},
	}

	component.StartMonitoringEndpoints()

	req, err := http.NewRequest("GET", "http://localhost:7876/healthz", nil)
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)

	assert.Equal(t, resp.StatusCode, 200)
	assert.Equal(t, resp.Header.Get("Content-Type"), "text/plain")
	body, err := ioutil.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, string(body), "ok")
}

func TestBadHealthzEndpoint(t *testing.T) {
	component := &Component{
		HealthMonitor:     BadHealthMonitor{},
		StatusPort:        9876,
		Type:              "loggregator",
		StatusCredentials: []string{"user", "pass"},
	}

	component.StartMonitoringEndpoints()

	req, err := http.NewRequest("GET", "http://localhost:9876/healthz", nil)
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)

	assert.Equal(t, resp.StatusCode, 200)
	body, err := ioutil.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, string(body), "bad")
}

type testInstrumentable struct {
	name    string
	metrics []instrumentation.Metric
}

func (t testInstrumentable) Emit() instrumentation.Context {
	return instrumentation.Context{t.name, t.metrics}
}

func TestVarzEndpoint(t *testing.T) {
	component := &Component{
		HealthMonitor:     GoodHealthMonitor{},
		StatusPort:        1234,
		Type:              "loggregator",
		StatusCredentials: []string{"user", "pass"},
		Instrumentables: []instrumentation.Instrumentable{
			testInstrumentable{
				"agentListener",
				[]instrumentation.Metric{
					instrumentation.Metric{"messagesReceived", 2004},
					instrumentation.Metric{"queueLength", 5},
				},
			},
			testInstrumentable{
				"cfSinkServer",
				[]instrumentation.Metric{
					instrumentation.Metric{"activeSinkCount", 3},
				},
			},
		},
	}

	component.StartMonitoringEndpoints()

	req, err := http.NewRequest("GET", "http://localhost:1234/varz", nil)
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)

	assert.Equal(t, resp.StatusCode, 200)
	assert.Equal(t, resp.Header.Get("Content-Type"), "application/json")
	body, err := ioutil.ReadAll(resp.Body)
	assert.NoError(t, err)

	expected := map[string]interface{}{
		"name":          "loggregator",
		"numCPUS":       runtime.NumCPU(),
		"numGoRoutines": runtime.NumGoroutine(),
		"contexts": []interface{}{
			map[string]interface{}{
				"name": "agentListener",
				"metrics": []interface{}{
					map[string]interface{}{
						"name":  "messagesReceived",
						"value": 2004,
					},
					map[string]interface{}{
						"name":  "queueLength",
						"value": 5,
					},
				},
			},
			map[string]interface{}{
				"name": "cfSinkServer",
				"metrics": []interface{}{
					map[string]interface{}{
						"name":  "activeSinkCount",
						"value": 3,
					},
				},
			},
		},
	}

	var actualMap map[string]interface{}
	json.Unmarshal(body, &actualMap)
	assert.Equal(t, expected, actualMap)
}
