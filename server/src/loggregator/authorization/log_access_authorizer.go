package authorization

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"io/ioutil"
	"loggregator/logtarget"
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
		Apps       []MetadataObject
	}

	type Space struct {
		Metadata    `json:"metadata"`
		SpaceEntity `json:"entity"`
	}

	type OrgEntity struct {
		Spaces   []Space
		Managers []MetadataObject
		Auditors []MetadataObject
	}

	type Org struct {
		Metadata  `json:"metadata"`
		OrgEntity `json:"entity"`
	}

	idIsInGroup := func(id string, group []MetadataObject) bool {
		for _, individual := range group {
			if individual.Guid == id {
				return true
			}
		}
		return false
	}

	getOrgFromCC := func(target *logtarget.LogTarget, authToken string) (o Org, err error) {
		client := &http.Client{}

		req, _ := http.NewRequest("GET", apiHost+"/v2/organizations/"+target.OrgId+"?inline-relations-depth=2", nil)
		req.Header.Set("Authorization", authToken)
		res, err := client.Do(req)
		if err != nil {
			err = errors.New(fmt.Sprintf("Could not get space information: [%s]", err))
			return
		}

		if res.StatusCode != 200 {
			err = errors.New(fmt.Sprintf("Non 200 response from CC API: %d", res.StatusCode))
			return
		}

		jsonBytes, err := ioutil.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			err = errors.New(fmt.Sprintf("Could not response body: %s", err))
			return
		}

		err = json.Unmarshal(jsonBytes, &o)
		if err != nil {
			err = errors.New(fmt.Sprintf("Error parsing organization JSON. json: %s\nerror: %s", jsonBytes, err))
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

		org, err := getOrgFromCC(target, authToken)

		if err != nil {
			logger.Errorf("Could not get Org info from CloudController: [%s]", err)
			return false
		}

		orgManagers := org.Managers
		orgAuditors := org.Auditors

		foundOrgManager := idIsInGroup(tokenPayload.UserId, orgManagers)
		foundOrgAuditor := idIsInGroup(tokenPayload.UserId, orgAuditors)

		if target.SpaceId == "" {
			return foundOrgManager || foundOrgAuditor
		}

		var targetSpace Space

		for _, space := range org.Spaces {
			if space.Guid == target.SpaceId {
				targetSpace = space
			}
		}
		if targetSpace.Guid == "" {
			return false
		}

		if target.AppId != "" && !idIsInGroup(target.AppId, targetSpace.Apps) {
			logger.Warnf("AppId (%s) not in space (%s)", target.AppId, target.SpaceId)
			return false
		}

		return foundOrgManager ||
			foundOrgAuditor ||
			idIsInGroup(tokenPayload.UserId, targetSpace.Managers) ||
			idIsInGroup(tokenPayload.UserId, targetSpace.Auditors) ||
			idIsInGroup(tokenPayload.UserId, targetSpace.Developers)
	}

	return LogAccessAuthorizer(authorizer)
}
