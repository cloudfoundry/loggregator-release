package authorization

import (
	"github.com/cloudfoundry/gosteno"
	"net/http"
)

type LogAccessAuthorizer func(authToken string, appId string, logger *gosteno.Logger) bool

func NewLogAccessAuthorizer(apiHost string) LogAccessAuthorizer {

	isAccessAllowed := func(target string, authToken string, logger *gosteno.Logger) bool {
		client := &http.Client{}

		req, _ := http.NewRequest("GET", apiHost+"/v2/apps/"+target, nil)
		req.Header.Set("Authorization", authToken)
		res, err := client.Do(req)
		if err != nil {
			logger.Errorf("Could not get app information: [%s]", err)
			return false
		}
		if res.StatusCode != 200 {
			logger.Warnf("Non 200 response from CC API: %d", res.StatusCode)
			return false
		}
		return true
	}

	authorizer := func(authToken string, appId string, logger *gosteno.Logger) bool {
		return isAccessAllowed(appId, authToken, logger)
	}

	return LogAccessAuthorizer(authorizer)
}
