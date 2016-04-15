package authorization

import (
	"errors"
	"net/http"

	"github.com/cloudfoundry/gosteno"
)

const (
	NO_AUTH_TOKEN_PROVIDED_ERROR_MESSAGE = "Error: Authorization not provided"
	INVALID_AUTH_TOKEN_ERROR_MESSAGE     = "Error: Invalid authorization"
)

type LogAccessAuthorizer func(authToken string, appId string, logger *gosteno.Logger) (bool, error)

func disableLogAccessControlAuthorizer(_, _ string, _ *gosteno.Logger) (bool, error) {
	return true, nil
}

func NewLogAccessAuthorizer(disableAccessControl bool, apiHost string) LogAccessAuthorizer {

	if disableAccessControl {
		return LogAccessAuthorizer(disableLogAccessControlAuthorizer)
	}

	isAccessAllowed := func(authToken string, target string, logger *gosteno.Logger) (bool, error) {
		if authToken == "" {
			return false, errors.New(NO_AUTH_TOKEN_PROVIDED_ERROR_MESSAGE)
		}

		req, _ := http.NewRequest("GET", apiHost+"/internal/log_access/"+target, nil)
		req.Header.Set("Authorization", authToken)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			logger.Errorf("Could not get app information: [%s]", err)
			return false, errors.New(INVALID_AUTH_TOKEN_ERROR_MESSAGE)
		}

		defer res.Body.Close()

		if res.StatusCode != 200 {
			logger.Warnf("Non 200 response from CC API: %d", res.StatusCode)
			return false, errors.New(INVALID_AUTH_TOKEN_ERROR_MESSAGE)
		}
		return true, nil
	}

	return LogAccessAuthorizer(isAccessAllowed)
}
