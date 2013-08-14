package sink

import (
	"encoding/json"
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

	authorizer := func(authToken string, target *logtarget.LogTarget, logger *gosteno.Logger) bool {
		tokenPayload, err := tokenDecoder.Decode(authToken)
		if err != nil {
			logger.Errorf("Could not decode auth token. %s", authToken)
			return false
		}

		client := &http.Client{}

		req, _ := http.NewRequest("GET", apiHost+"/v2/organizations/"+target.OrgId+"?inline-relations-depth=2", nil)
		req.Header.Set("Authorization", authToken)
		res, err := client.Do(req)
		if err != nil {
			logger.Errorf("Could not get space information: [%s]", err)
			return false
		}

		if res.StatusCode != 200 {
			logger.Warnf("Non 200 response from CC API: %d", res.StatusCode)
			return false
		}

		jsonBytes, err := ioutil.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			logger.Errorf("Could not response body: %s", err)
			return false
		}

		var org Org
		err = json.Unmarshal(jsonBytes, &org)
		if err != nil {
			logger.Errorf("Error parsing organization JSON. json: %s\nerror: %s", jsonBytes, err)
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
