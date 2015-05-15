package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"crypto/tls"
	"syslog_drain_binder/shared_types"
)

func Poll(hostname string, username string, password string, batchSize int, skipCertVerify bool) (map[shared_types.AppId][]shared_types.DrainURL, error) {
	drainURLs := make(map[shared_types.AppId][]shared_types.DrainURL)

	nextId := 0

	tr := &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: skipCertVerify},
		DisableKeepAlives: true,
	}
	client := &http.Client{Transport: tr}

	for {
		url := buildUrl(hostname, batchSize, nextId)
		request, _ := http.NewRequest("GET", url, nil)
		request.SetBasicAuth(username, password)

		ccResponse, err := pollAndDecode(client, request)
		if err != nil {
			return drainURLs, err
		}

		for appId, urls := range ccResponse.Results {
			drainURLs[appId] = urls
		}

		if ccResponse.NextId == nil {
			break
		}
		nextId = *ccResponse.NextId
	}

	return drainURLs, nil
}

func pollAndDecode(client *http.Client, request *http.Request) (*cloudControllerResponse, error) {
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("Remote server error: %s", http.StatusText(response.StatusCode)))
	}

	decoder := json.NewDecoder(response.Body)
	var ccResponse cloudControllerResponse
	decoder.Decode(&ccResponse)

	return &ccResponse, nil
}

type cloudControllerResponse struct {
	Results map[shared_types.AppId][]shared_types.DrainURL `json:"results"`
	NextId  *int                                           `json:"next_id"`
}

func buildUrl(baseURL string, batchSize int, nextId int) string {
	url := fmt.Sprintf("%s/v2/syslog_drain_urls?batch_size=%d", baseURL, batchSize)

	if nextId != 0 {
		url = fmt.Sprintf("%s&next_id=%d", url, nextId)
	}
	return url
}
