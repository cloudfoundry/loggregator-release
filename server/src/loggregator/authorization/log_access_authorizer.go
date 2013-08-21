package authorization

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/logtarget"
	"io/ioutil"
	"net/http"
)

type LogAccessAuthorizer func(authToken string, target *logtarget.LogTarget, logger *gosteno.Logger) bool

func NewLogAccessAuthorizer(tokenDecoder TokenDecoder, apiHost string) LogAccessAuthorizer {
	type Metadata struct {
		Guid string
	}

	type MetadataObject struct {
		Metadata `json:"metadata"`
	}

	type SpaceEntity struct {
		Developers []MetadataObject
		Managers   []MetadataObject
		Auditors   []MetadataObject
	}

	type Space struct {
		Metadata    `json:"metadata"`
		SpaceEntity `json:"entity"`
	}

	type AppEntity struct {
		Space Space
	}

	type App struct {
		Metadata  `json:"metadata"`
		AppEntity `json:"entity"`
	}

	idIsInGroup := func(id string, group []MetadataObject) bool {
		for _, individual := range group {
			if individual.Guid == id {
				return true
			}
		}
		return false
	}

	getAppFromCC := func(target *logtarget.LogTarget, authToken string) (a App, error error) {
		client := &http.Client{}

		req, _ := http.NewRequest("GET", apiHost+"/v2/apps/"+target.AppId+"?inline-relations-depth=2", nil)
		req.Header.Set("Authorization", authToken)
		res, err := client.Do(req)
		if err != nil {
			err = errors.New(fmt.Sprintf("Could not get app information: [%s]", err))
			return
		}

		if res.StatusCode != 200 {
			err = errors.New(fmt.Sprintf("Non 200 response from CC API: %d", res.StatusCode))
			return
		}

		jsonBytes, err := ioutil.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			err = errors.New(fmt.Sprintf("Could not read response body: %s", err))
			return
		}

		err = json.Unmarshal(jsonBytes, &a)
		if err != nil {
			err = errors.New(fmt.Sprintf("Error parsing application JSON. json: %s\nerror: %s", jsonBytes, err))
			return
		}
		return
	}

	authorizer := func(authToken string, target *logtarget.LogTarget, logger *gosteno.Logger) bool {
		tokenPayload, err := tokenDecoder.Decode(authToken)
		if err != nil {
			logger.Errorf("Could not decode auth token. %s", authToken)
			return false
		}

		app, err := getAppFromCC(target, authToken)
		if err != nil {
			logger.Errorf("Could not get Application info from CloudController: [%s]", err)
			return false
		}

		return idIsInGroup(tokenPayload.UserId, app.Space.Managers) ||
			idIsInGroup(tokenPayload.UserId, app.Space.Auditors) ||
			idIsInGroup(tokenPayload.UserId, app.Space.Developers)
	}

	return LogAccessAuthorizer(authorizer)
}
