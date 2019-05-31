package auth

import (
	"errors"
	"log"
	"net/http"
)

var (
	ErrNoAuthTokenProvided = errors.New("Error: Authorization not provided")
	ErrInvalidAuthToken    = errors.New("Error: Invalid authorization")
)

// TODO: We don't need to return an error and a status code. One will suffice.
type LogAccessAuthorizer func(authToken string, appId string) (int, error)

func disableLogAccessControlAuthorizer(_, _ string) (int, error) {
	return http.StatusOK, nil
}

func NewLogAccessAuthorizer(c *http.Client, disableAccessControl bool, apiHost string) LogAccessAuthorizer {
	if disableAccessControl {
		return LogAccessAuthorizer(disableLogAccessControlAuthorizer)
	}

	return LogAccessAuthorizer(func(authToken string, target string) (int, error) {
		if authToken == "" {
			log.Println(ErrNoAuthTokenProvided)
			return http.StatusUnauthorized, ErrNoAuthTokenProvided
		}

		req, err := http.NewRequest("GET", apiHost+"/internal/v4/log_access/"+target, nil)
		if err != nil {
			return http.StatusInternalServerError, err
		}

		req.Header.Set("Authorization", authToken)
		res, err := c.Do(req)
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
	})
}
