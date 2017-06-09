package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"code.cloudfoundry.org/loggregator/syslog_drain_binder/shared_types"
)

const urlPathFmt = "%s/internal/v4/syslog_drain_urls?batch_size=%d"

// DefaultTimeout is the default http client timeout used when polling.
var DefaultTimeout = 5 * time.Second

// PollOptions contains the options for the Poll function.
type PollOptions struct {
	insecureSkipVerify bool
	timeout            time.Duration
}

// Timeout specifies the http client timeout when polling.
func Timeout(t time.Duration) func(*PollOptions) {
	return func(o *PollOptions) {
		o.timeout = t
	}
}

type cloudControllerResponse struct {
	Results map[shared_types.AppID]shared_types.SyslogDrainBinding `json:"results"`
	NextID  *int                                                   `json:"next_id"`
}

// Poll gets all the app's syslog drain urls from the cloud controller.
func Poll(
	urlBase string,
	batchSize int,
	tlsConfig *tls.Config,
	options ...func(*PollOptions),
) (shared_types.AllSyslogDrainBindings, error) {
	drainURLs := make(shared_types.AllSyslogDrainBindings)
	nextID := 0

	opts := PollOptions{
		timeout: DefaultTimeout,
	}
	for _, o := range options {
		o(&opts)
	}

	tr := &http.Transport{
		TLSClientConfig:   tlsConfig,
		DisableKeepAlives: true,
	}
	client := &http.Client{
		Timeout:   opts.timeout,
		Transport: tr,
	}

	for {
		url := buildUrl(urlBase, batchSize, nextID)
		request, _ := http.NewRequest("GET", url, nil)

		ccResponse, err := pollAndDecode(client, request)
		if err != nil {
			return drainURLs, err
		}

		for appID, urls := range ccResponse.Results {
			drainURLs[appID] = urls
		}

		if ccResponse.NextID == nil {
			break
		}
		nextID = *ccResponse.NextID
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

func buildUrl(baseURL string, batchSize int, nextID int) string {
	url := fmt.Sprintf(urlPathFmt, baseURL, batchSize)

	if nextID != 0 {
		url = fmt.Sprintf("%s&next_id=%d", url, nextID)
	}
	return url
}
