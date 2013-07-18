package cfsink

import (
	"github.com/bitly/simplejson"
	"github.com/cloudfoundry/gosteno"
	"io/ioutil"
	"net/http"
)

type LogAccessAuthorizer func(apiHost, authToken, spaceId, appId string, logger *gosteno.Logger) bool

func NewLogAccessAuthorizer(tokenDecoder TokenDecoder) LogAccessAuthorizer {
	authorizer := func(apiHost, authToken, spaceId, appId string, logger *gosteno.Logger) bool {
		idIsInGroup := func(id string, group []interface{}) bool {
			for _, individual := range group {
				metadata := individual.(map[string]interface{})["metadata"]
				guid := metadata.(map[string]interface{})["guid"]

				if guid == id {
					return true
				}

			}
			return false
		}

		decodedInformation, err := tokenDecoder.Decode(authToken)

		if err != nil {
			logger.Errorf("Could not decode auth token. %s", authToken)
			return false
		}
		userId, found := decodedInformation["user_id"].(string)

		if !found {
			logger.Errorf("No User ID found in the auth token. %v", decodedInformation)
			return false
		}

		client := &http.Client{}

		req, _ := http.NewRequest("GET", apiHost+"/v2/spaces/"+spaceId+"?inline-relations-depth=1", nil)
		req.Header.Set("Authorization", authToken)
		res, err := client.Do(req)
		if err != nil {
			logger.Errorf("Could not get space information: %s", err)
			return false
		}

		if res.StatusCode != 200 {
			logger.Warnf("Non 200 response from CC API: %d", res.StatusCode)
			return false
		}

		json, err := ioutil.ReadAll(res.Body)
		res.Body.Close()

		if err != nil {
			logger.Errorf("Could not read space json: %s", err)
			return false
		}

		js, err := simplejson.NewJson(json)
		res.Body.Close()

		if err != nil {
			logger.Errorf("Error parsing space JSON. json: %s\nerror: %s", json, err)
			return false
		}

		logger.Debugf("JSON: %s\nuserId: %s", json, userId)

		apps, err := js.Get("entity").Get("apps").Array()
		if err != nil {
			logger.Errorf("Error getting apps from space JSON: %s, err: %s", json, err)
			return false
		}

		if appId != "" && !idIsInGroup(appId, apps) {
			logger.Warnf("AppId (%s) not in space (%s)", appId, spaceId)
			return false
		}

		managers, err := js.Get("entity").Get("managers").Array()
		if err != nil {
			logger.Errorf("Error getting managers from space JSON: %s, err: %s", json, err)
			return false
		}

		auditors, err := js.Get("entity").Get("auditors").Array()
		if err != nil {
			logger.Errorf("Error getting auditors from space JSON: %s, err: %s", json, err)
			return false
		}

		developers, err := js.Get("entity").Get("developers").Array()
		if err != nil {
			logger.Errorf("Error getting developers from space JSON: %s, err: %s", json, err)
			return false
		}

		return idIsInGroup(userId, managers) || idIsInGroup(userId, auditors) || idIsInGroup(userId, developers)
	}

	return LogAccessAuthorizer(authorizer)
}
