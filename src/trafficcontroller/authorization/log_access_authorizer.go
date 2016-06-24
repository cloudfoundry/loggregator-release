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

type LogAccessAuthorizer func(authToken string, appId string, logger *gosteno.Logger) (int, error)

func disableLogAccessControlAuthorizer(_, _ string, _ *gosteno.Logger) (int, error) {
	return http.StatusOK, nil
}

func NewLogAccessAuthorizer(disableAccessControl bool, apiHost string) LogAccessAuthorizer {

	if disableAccessControl {
		return LogAccessAuthorizer(disableLogAccessControlAuthorizer)
	}

	isAccessAllowed := func(authToken string, target string, logger *gosteno.Logger) (int, error) {
		if authToken == "" {
			logger.Errorf(NO_AUTH_TOKEN_PROVIDED_ERROR_MESSAGE)
			return http.StatusUnauthorized, errors.New(NO_AUTH_TOKEN_PROVIDED_ERROR_MESSAGE)
		}

		req, _ := http.NewRequest("GET", apiHost+"/internal/log_access/"+target, nil)
		req.Header.Set("Authorization", authToken)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			logger.Errorf("Could not get app information: [%s]", err)
			return http.StatusInternalServerError, err
		}

		defer res.Body.Close()

		err = nil
		if res.StatusCode != 200 {
			logger.Warnf("Non 200 response from CC API: %d for %s", res.StatusCode, target)
			err = errors.New(http.StatusText(res.StatusCode))
		}

		return res.StatusCode, err
	}

	return LogAccessAuthorizer(isAccessAllowed)
}
