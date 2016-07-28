package cflib

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
)

type UAA struct {
	URL          string
	Username     string
	Password     string
	ClientID     string
	ClientSecret string
}

func (u *UAA) GetAuthToken() (string, error) {
	uaaURL := fmt.Sprintf("%s/oauth/token", u.URL)
	data := url.Values{}
	data.Set("grant_type", "password")
	data.Set("client_id", u.ClientID)
	data.Set("client_secret", u.ClientSecret)
	data.Set("username", u.Username)
	data.Set("password", u.Password)
	data.Set("response_type", "token")
	data.Set("scope", "")

	r, err := http.NewRequest("POST", uaaURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return "", err
	}
	basicAuthToken := base64.StdEncoding.EncodeToString([]byte(u.ClientID + ":" + u.ClientSecret))
	r.Header.Set("Authorization", "Basic "+basicAuthToken)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		errout, _ := ioutil.ReadAll(resp.Body)
		log.Printf("StatusCode: %d\n Error: %s\n", resp.StatusCode, errout)
		return "", errors.New("response not 200")
	}

	defer resp.Body.Close()
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	um := &struct {
		AccessToken string `json:"access_token"`
	}{}
	json.Unmarshal(content, um)

	return um.AccessToken, nil
}

type CC struct {
	URL string
}

func (cc *CC) GetAppName(guid, authToken string) string {
	if authToken == "" {
		return guid
	}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v2/apps/%s", cc.URL, guid), nil)
	if err != nil {
		log.Printf("Error building request: %s\n", err)
		return guid
	}

	req.Header.Set("Authorization", fmt.Sprintf("bearer %s", authToken))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Error getting app name: %s", err)
		return guid
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Printf("Got status %s while getting app name\n", resp.Status)
		return guid
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body from CC: %s\n", err)
		return guid
	}

	type Entity struct {
		Name string `json:"name"`
	}
	um := &struct {
		Entity Entity `json:"entity"`
	}{}

	json.Unmarshal(body, um)
	return um.Entity.Name
}
