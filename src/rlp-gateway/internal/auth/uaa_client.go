package auth

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Metrics defines the interface for creating metrics
type Metrics interface {
	NewGauge(name string) func(value float64)
}

// HTTClient defines the interface for communication over HTTP
type HTTPClient interface {
	Do(r *http.Request) (*http.Response, error)
}

// Oauth2Client defines an OAuth2 client
type Oauth2Client struct {
	IsAdmin  bool
	ClientID string
	UserID   string
}

// UAAClient defines an http client for UAA
type UAAClient struct {
	httpClient   HTTPClient
	uaa          *url.URL
	client       string
	clientSecret string
	storeLatency func(float64)
}

// NewUAAClient returns a UAA client
func NewUAAClient(
	uaaAddr string,
	client string,
	clientSecret string,
	httpClient HTTPClient,
	m Metrics,
	log *log.Logger,
) *UAAClient {
	u, err := url.Parse(uaaAddr)
	if err != nil {
		log.Fatalf("failed to parse UAA addr: %s", err)
	}

	u.Path = "check_token"

	return &UAAClient{
		uaa:          u,
		client:       client,
		clientSecret: clientSecret,
		httpClient:   httpClient,
		storeLatency: m.NewGauge("LastUAALatency"),
	}
}

// Read retrieves an OAuth2 client from UAA
func (c *UAAClient) Read(token string) (Oauth2Client, error) {
	if token == "" {
		return Oauth2Client{}, errors.New("missing token")
	}

	form := url.Values{
		"token": {trimBearer(token)},
	}

	req, err := http.NewRequest("POST", c.uaa.String(), strings.NewReader(form.Encode()))
	if err != nil {
		log.Printf("failed to create UAA request: %s", err)
		return Oauth2Client{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(c.client, c.clientSecret)

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	c.storeLatency(float64(time.Since(start)))
	if err != nil {
		log.Printf("UAA request failed: %s", err)
		return Oauth2Client{}, err
	}

	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()

	uaaR, err := c.parseResponse(resp.Body)
	if err != nil {
		log.Printf("failed to parse UAA response body: %s", err)
		return Oauth2Client{}, err
	}

	return Oauth2Client{
		IsAdmin:  resp.StatusCode == http.StatusOK && c.hasDopplerScope(uaaR),
		UserID:   uaaR.UserID,
		ClientID: uaaR.ClientID,
	}, nil
}

func trimBearer(authToken string) string {
	return strings.TrimSpace(strings.TrimPrefix(authToken, "bearer"))
}

type uaaResponse struct {
	Scopes   []string `json:"scope"`
	UserID   string   `json:"user_id"`
	ClientID string   `json:"client_id"`
}

func (c *UAAClient) hasDopplerScope(r uaaResponse) bool {
	for _, scope := range r.Scopes {
		if scope == "doppler.firehose" || scope == "logs.admin" {
			return true
		}
	}

	return false
}

func (c *UAAClient) parseResponse(r io.Reader) (uaaResponse, error) {
	var resp uaaResponse
	if err := json.NewDecoder(r).Decode(&resp); err != nil {
		log.Printf("unable to decode json response from UAA: %s", err)
		return uaaResponse{}, err
	}
	return resp, nil
}
