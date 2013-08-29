package authorization

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/logtarget"
	"net/http"
)

type LogAccessAuthorizer func(authToken string, target *logtarget.LogTarget, logger *gosteno.Logger) bool

func NewLogAccessAuthorizer(tokenDecoder TokenDecoder, apiHost string) LogAccessAuthorizer {
	userIsInLoggregatorGroup := func(scopes []string) bool {
		for _, scope := range scopes {
			if scope == "loggregator" {
				return true
			}
		}
		return false
	}

	isAccessAllowed := func(target *logtarget.LogTarget, authToken string, logger *gosteno.Logger) bool {
		client := &http.Client{}

		req, _ := http.NewRequest("GET", apiHost+"/v2/apps/"+target.AppId, nil)
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

	authorizer := func(authToken string, target *logtarget.LogTarget, logger *gosteno.Logger) bool {
		tokenPayload, err := tokenDecoder.Decode(authToken)
		if err != nil {
			logger.Errorf("Could not decode auth token. %s", authToken)
			return false
		}

		if !isAccessAllowed(target, authToken, logger) {
			return false
		}

		if !userIsInLoggregatorGroup(tokenPayload.Scope) {
			logger.Errorf("Only users in 'loggregator' scope can access logs")
			return false
		}

		return true
	}

	return LogAccessAuthorizer(authorizer)
}
