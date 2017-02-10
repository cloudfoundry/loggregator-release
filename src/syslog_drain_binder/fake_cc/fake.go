package fake_cc

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

type AppEntry struct {
	AppId         string
	SyslogBinding SysLogBinding
}

type SysLogBinding struct {
	Hostname  string   `json:"hostname"`
	DrainURLs []string `json:"drains"`
}

type jsonResponse struct {
	Results map[string]SysLogBinding `json:"results"`
	NextId  *int                     `json:"next_id"`
}

type FakeCC struct {
	ServedRoute  string
	QueryParams  url.Values
	RequestCount int
	FailOn       int
	appDrains    []AppEntry
}

func NewFakeCC(appDrains []AppEntry) *FakeCC {
	return &FakeCC{
		appDrains: appDrains,
	}
}

func (fake *FakeCC) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if fake.FailOn > 0 && fake.RequestCount >= fake.FailOn {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	fake.RequestCount++
	fake.ServedRoute = r.URL.Path
	fake.QueryParams = r.URL.Query()

	batchSize, _ := strconv.Atoi(fake.QueryParams.Get("batch_size"))
	start, _ := strconv.Atoi(fake.QueryParams.Get("next_id"))

	w.Write(fake.buildResponse(start, start+batchSize))
}

func (fake *FakeCC) buildResponse(start int, end int) []byte {
	var r jsonResponse
	if start >= len(fake.appDrains) {
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

	for i := start; i < end && i < len(fake.appDrains); i++ {
		r.Results[fake.appDrains[i].AppId] = fake.appDrains[i].SyslogBinding
	}

	b, _ := json.Marshal(r)
	return b
}
