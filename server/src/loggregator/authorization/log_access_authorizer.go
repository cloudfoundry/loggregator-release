package authorization

import (
	"github.com/cloudfoundry/gosteno"
	"net/http"
	"regexp"
)

type LogAccessAuthorizer func(authToken string, appId string, logger *gosteno.Logger) bool

func NewLogAccessAuthorizer(tokenDecoder TokenDecoder, apiHost string) LogAccessAuthorizer {
	userIsInAllowedEmailDomain := func(email string) bool {
		re := regexp.MustCompile("(?i)^[^@]+@(vmware.com|pivotallabs.com|gopivotal.com)$")
		return re.MatchString(email)
	}

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
		tokenPayload, err := tokenDecoder.Decode(authToken)
		if err != nil {
			logger.Errorf("Could not decode auth token. %s", authToken)
			return false
		}

		if !isAccessAllowed(appId, authToken, logger) {
			return false
		}

		if !userIsInAllowedEmailDomain(tokenPayload.Email) {
			logger.Errorf("Only users in 'loggregator' scope can access logs")
			return false
		}

		return true
	}

	return LogAccessAuthorizer(authorizer)
}
