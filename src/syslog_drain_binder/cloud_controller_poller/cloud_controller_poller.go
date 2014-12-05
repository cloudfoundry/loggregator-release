package cloud_controller_poller

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"errors"
)

type AppId string
type DrainURL string


func Poll(hostname net.Addr, username string, password string, batchSize int) (map[AppId][]DrainURL, error) {
	drainURLs := make(map[AppId][]DrainURL)

	nextId := 0

	for {
		url := buildUrl(hostname.String(), batchSize, nextId)
		request, _ := http.NewRequest("GET", url, nil)
		request.SetBasicAuth(username, password)

		response, err := http.DefaultClient.Do(request)
		if err != nil {
			return drainURLs, err
		}

		if response.StatusCode != http.StatusOK {
			return drainURLs, errors.New(fmt.Sprintf("Remote server error: %s", http.StatusText(response.StatusCode)))
		}


		decoder := json.NewDecoder(response.Body)
		var ccResponse cloudControllerResponse
		decoder.Decode(&ccResponse)

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

type cloudControllerResponse struct {
	Results map[AppId][]DrainURL `json:"results"`
	NextId  *int                 `json:"next_id"`
}

func buildUrl(baseURL string, batchSize int, nextId int) string {
	url := fmt.Sprintf("http://%s/v2/syslog_drain_urls", baseURL)
	url = fmt.Sprintf("%s?batch_size=%d", url, batchSize)

	if nextId != 0 {
		url = fmt.Sprintf("%s&next_id=%d", url, nextId)
	}
	return url
}
