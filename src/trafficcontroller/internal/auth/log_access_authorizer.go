package auth

import (
	"errors"
	"log"
	"net/http"
)

const (
	NO_AUTH_TOKEN_PROVIDED_ERROR_MESSAGE = "Error: Authorization not provided"
	INVALID_AUTH_TOKEN_ERROR_MESSAGE     = "Error: Invalid authorization"
)

// TODO: We don't need to return an error and a status code. One will suffice.
type LogAccessAuthorizer func(authToken string, appId string) (int, error)

func disableLogAccessControlAuthorizer(_, _ string) (int, error) {
	return http.StatusOK, nil
}

func NewLogAccessAuthorizer(disableAccessControl bool, apiHost string) LogAccessAuthorizer {

	if disableAccessControl {
		return LogAccessAuthorizer(disableLogAccessControlAuthorizer)
	}

	isAccessAllowed := func(authToken string, target string) (int, error) {
		if authToken == "" {
			log.Printf(NO_AUTH_TOKEN_PROVIDED_ERROR_MESSAGE)
			return http.StatusUnauthorized, errors.New(NO_AUTH_TOKEN_PROVIDED_ERROR_MESSAGE)
		}

		req, err := http.NewRequest("GET", apiHost+"/internal/log_access/"+target, nil)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		req.Header.Set("Authorization", authToken)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("Could not get app information: [%s]", err)
			return http.StatusInternalServerError, err
		}

		defer res.Body.Close()

		err = nil
		if res.StatusCode != 200 {
			log.Printf("Non 200 response from CC API: %d for %q", res.StatusCode, target)
			err = errors.New(http.StatusText(res.StatusCode))
		}

		return res.StatusCode, err
	}

	return LogAccessAuthorizer(isAccessAllowed)
}
