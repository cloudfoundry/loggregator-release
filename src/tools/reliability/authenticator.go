package reliability

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

type UAAClient struct {
	clientID     string
	clientSecret string
	uaaAddr      string
	httpClient   *http.Client
}

func NewUAAClient(id, secret, uaaAddr string, c *http.Client) *UAAClient {
	return &UAAClient{
		clientID:     id,
		clientSecret: secret,
		uaaAddr:      uaaAddr,
		httpClient:   c,
	}
}

func (a *UAAClient) Token() (string, error) {
	response, err := a.httpClient.PostForm(a.uaaAddr+"/oauth/token", url.Values{
		"response_type": []string{"token"},
		"grant_type":    []string{"client_credentials"},
		"client_id":     []string{a.clientID},
		"client_secret": []string{a.clientSecret},
	})
	if err != nil {
		return "", err
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Expected 200 status code from /oauth/token, got %d", response.StatusCode)
	}

	body, err := ioutil.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		return "", err
	}

	oauthResponse := make(map[string]interface{})
	err = json.Unmarshal(body, &oauthResponse)
	if err != nil {
		return "", err
	}

	accessTokenInterface, ok := oauthResponse["access_token"]
	if !ok {
		return "", errors.New("No access_token on UAA oauth response")
	}

	accessToken, ok := accessTokenInterface.(string)
	if !ok {
		return "", errors.New("access_token on UAA oauth response not a string")
	}

	return "bearer " + accessToken, nil
}
