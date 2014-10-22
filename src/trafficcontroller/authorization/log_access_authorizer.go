package authorization

import (
	"crypto/tls"
	"errors"
	"github.com/cloudfoundry/gosteno"
	"net/http"
)

const (
	NO_AUTH_TOKEN_PROVIDED_ERROR_MESSAGE = "Error: Authorization not provided"
	INVALID_AUTH_TOKEN_ERROR_MESSAGE     = "Error: Invalid authorization"
)

type LogAccessAuthorizer func(authToken string, appId string, logger *gosteno.Logger) (bool, error)

func disableLogAccessControlAuthorizer(_, _ string, _ *gosteno.Logger) (bool, error) {
	return true, nil
}

func NewLogAccessAuthorizer(disableAccessControl bool, apiHost string, skipCertVerify bool) LogAccessAuthorizer {

	if disableAccessControl {
		return LogAccessAuthorizer(disableLogAccessControlAuthorizer)
	}

	isAccessAllowed := func(authToken string, target string, logger *gosteno.Logger) (bool, error) {
		if authToken == "" {
			return false, errors.New(NO_AUTH_TOKEN_PROVIDED_ERROR_MESSAGE)
		}

		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: skipCertVerify},
		}
		client := &http.Client{Transport: tr}

		req, _ := http.NewRequest("GET", apiHost+"/v2/apps/"+target, nil)
		req.Header.Set("Authorization", authToken)
		res, err := client.Do(req)
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
