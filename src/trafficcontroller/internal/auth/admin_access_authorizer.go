package auth

import (
	"errors"
	"log"
	"strings"
)

const LOGGREGATOR_ADMIN_ROLE = "doppler.firehose"
const BEARER_PREFIX = "bearer "

type AdminAccessAuthorizer func(authToken string) (bool, error)

func disableAdminAccessControlAuthorizer(string) (bool, error) {
	return true, nil
}

func NewAdminAccessAuthorizer(disableAccessControl bool, client UaaClient) AdminAccessAuthorizer {

	if disableAccessControl {
		return AdminAccessAuthorizer(disableAdminAccessControlAuthorizer)
	}

	isAccessAllowed := func(authToken string) (bool, error) {
		if authToken == "" {
			return false, errors.New(NO_AUTH_TOKEN_PROVIDED_ERROR_MESSAGE)
		}

		authData, err := client.GetAuthData(strings.TrimPrefix(authToken, BEARER_PREFIX))

		if err != nil {
			log.Printf("Error getting auth data: %s", err)
			return false, errors.New(INVALID_AUTH_TOKEN_ERROR_MESSAGE)
		}

		if authData.HasPermission(LOGGREGATOR_ADMIN_ROLE) {
			return true, nil
		} else {
			return false, errors.New(INVALID_AUTH_TOKEN_ERROR_MESSAGE)
		}
	}

	return AdminAccessAuthorizer(isAccessAllowed)
}
