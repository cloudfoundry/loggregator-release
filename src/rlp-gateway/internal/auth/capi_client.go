package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"
)

// CAPIClient defines a client for accessing the CC API
type CAPIClient struct {
	client                           HTTPClient
	capi                             string
	externalCapi                     string
	storeAppsLatency                 func(float64)
	storeListServiceInstancesLatency func(float64)
	storeLogAccessLatency            func(float64)
	storeServiceInstancesLatency     func(float64)
}

// NewCAPIClient returns a new CAPIClient
func NewCAPIClient(
	capiAddr string,
	externalCapiAddr string,
	client HTTPClient,
	m Metrics,
	log *log.Logger,
) *CAPIClient {
	_, err := url.Parse(capiAddr)
	if err != nil {
		log.Fatalf("failed to parse internal CAPI addr: %s", err)
	}

	_, err = url.Parse(externalCapiAddr)
	if err != nil {
		log.Fatalf("failed to parse external CAPI addr: %s", err)
	}

	return &CAPIClient{
		client:                           client,
		capi:                             capiAddr,
		externalCapi:                     externalCapiAddr,
		storeAppsLatency:                 m.NewGauge("LastCAPIV3AppsLatency"),
		storeListServiceInstancesLatency: m.NewGauge("LastCAPIV2ListServiceInstancesLatency"),
		storeLogAccessLatency:            m.NewGauge("LastCAPIV4LogAccessLatency"),
		storeServiceInstancesLatency:     m.NewGauge("LastCAPIV2ServiceInstancesLatency"),
	}
}

// IsAuthorized determines autorization to a given sourceID is valid
func (c *CAPIClient) IsAuthorized(sourceID, token string) bool {
	uri := fmt.Sprintf("%s/internal/v4/log_access/%s", c.capi, sourceID)
	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		log.Printf("failed to build authorize log access request: %s", err)
		return false
	}

	req.Header.Set("Authorization", token)
	start := time.Now()
	resp, err := c.client.Do(req)
	c.storeLogAccessLatency(float64(time.Since(start)))

	if err != nil {
		log.Printf("CAPI request failed: %s", err)
		return false
	}

	defer func(r *http.Response) {
		cleanup(r)
	}(resp)

	if resp.StatusCode == http.StatusOK {
		return true
	}

	uri = fmt.Sprintf("%s/v2/service_instances/%s", c.externalCapi, sourceID)
	req, err = http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		log.Printf("failed to build authorize service instance access request: %s", err)
		return false
	}

	req.Header.Set("Authorization", token)
	start = time.Now()
	resp, err = c.client.Do(req)
	c.storeServiceInstancesLatency(float64(time.Since(start)))
	if err != nil {
		log.Printf("External CAPI request failed: %s", err)
		return false
	}

	defer func(r *http.Response) {
		cleanup(r)
	}(resp)

	return resp.StatusCode == http.StatusOK
}

// AvailableSourceIDs returns all the available source ids a client has access to
func (c *CAPIClient) AvailableSourceIDs(token string) []string {
	var sourceIDs []string
	req, err := http.NewRequest(http.MethodGet, c.externalCapi+"/v3/apps", nil)
	if err != nil {
		log.Printf("failed to build authorize log access request: %s", err)
		return nil
	}

	req.Header.Set("Authorization", token)
	start := time.Now()
	resp, err := c.client.Do(req)
	c.storeAppsLatency(float64(time.Since(start)))
	if err != nil {
		log.Printf("CAPI request failed: %s", err)
		return nil
	}

	defer func(r *http.Response) {
		cleanup(r)
	}(resp)

	if resp.StatusCode != http.StatusOK {
		log.Printf("CAPI request failed (/v3/apps): %d", resp.StatusCode)
		return nil
	}

	var appSources struct {
		Resources []struct {
			Guid string `json:"guid"`
		} `json:"resources"`
	}

	json.NewDecoder(resp.Body).Decode(&appSources)

	for _, v := range appSources.Resources {
		sourceIDs = append(sourceIDs, v.Guid)
	}

	req, err = http.NewRequest(http.MethodGet, c.externalCapi+"/v2/service_instances", nil)
	if err != nil {
		log.Printf("failed to build authorize service instance access request: %s", err)
		return nil
	}

	req.Header.Set("Authorization", token)
	start = time.Now()
	resp, err = c.client.Do(req)
	c.storeListServiceInstancesLatency(float64(time.Since(start)))
	if err != nil {
		log.Printf("External CAPI request failed: %s", err)
		return nil
	}

	defer func(r *http.Response) {
		cleanup(r)
	}(resp)

	if resp.StatusCode != http.StatusOK {
		log.Printf("CAPI request failed (/v2/service_instances): %d", resp.StatusCode)
		return nil
	}

	var serviceSources struct {
		Resources []struct {
			Metadata struct {
				Guid string `json:"guid"`
			} `json:"metadata"`
		} `json:"resources"`
	}

	json.NewDecoder(resp.Body).Decode(&serviceSources)

	for _, v := range serviceSources.Resources {
		sourceIDs = append(sourceIDs, v.Metadata.Guid)
	}

	return sourceIDs
}

func cleanup(resp *http.Response) {
	io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()
}
