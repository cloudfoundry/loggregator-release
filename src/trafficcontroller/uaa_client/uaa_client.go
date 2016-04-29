package uaa_client

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

type UaaClient interface {
	GetAuthData(token string) (*AuthData, error)
}

type uaaClient struct {
	address string
	id      string
	secret  string
}

func NewUaaClient(address, id, secret string) uaaClient {

	return uaaClient{
		address: address,
		id:      id,
		secret:  secret,
	}
}

func (client *uaaClient) GetAuthData(token string) (*AuthData, error) {

	formValues := url.Values{"token": []string{token}}
	req, _ := http.NewRequest("POST", client.address+"/check_token", strings.NewReader(formValues.Encode()))
	req.SetBasicAuth(client.id, client.secret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusUnauthorized {
		return nil, errors.New("Invalid username/password")
	}

	if response.StatusCode == http.StatusNotFound {
		return nil, errors.New("API endpoint not found")
	}

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	if response.StatusCode == http.StatusBadRequest {
		var uaaError uaaErrorResponse
		json.Unmarshal(responseBody, &uaaError)
		return nil, errors.New(uaaError.ErrorDescription)
	}

	if response.StatusCode != http.StatusOK {
		return nil, errors.New("Unknown error occurred")
	}

	var aData AuthData
	err = json.Unmarshal(responseBody, &aData)

	if err != nil {
		return nil, err
	}

	return &aData, nil
}

type uaaErrorResponse struct {
	ErrorDescription string `json:"error_description"`
}

type AuthData struct {
	Scope []string `json:"scope"`
}

func (data *AuthData) HasPermission(perm string) bool {
	for _, permissionName := range data.Scope {
		if permissionName == perm {
			return true
		}
	}

	return false
}
