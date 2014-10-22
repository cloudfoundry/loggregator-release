package authorization

import (
	"errors"
	"github.com/cloudfoundry/gosteno"
	"strings"
	"trafficcontroller/uaa_client"
)

const LOGGREGATOR_ADMIN_ROLE = "doppler.firehose"
const BEARER_PREFIX = "bearer "

type AdminAccessAuthorizer func(authToken string, logger *gosteno.Logger) (bool, error)

func disableAdminAccessControlAuthorizer(_ string, _ *gosteno.Logger) (bool, error) {
	return true, nil
}

func NewAdminAccessAuthorizer(disableAccessControl bool, client uaa_client.UaaClient) AdminAccessAuthorizer {

	if disableAccessControl {
		return AdminAccessAuthorizer(disableAdminAccessControlAuthorizer)
	}

	isAccessAllowed := func(authToken string, logger *gosteno.Logger) (bool, error) {
		if authToken == "" {
			return false, errors.New(NO_AUTH_TOKEN_PROVIDED_ERROR_MESSAGE)
		}

		authData, err := client.GetAuthData(strings.TrimPrefix(authToken, BEARER_PREFIX))

		if err != nil {
			logger.Errorf("Error getting auth data: %s", err.Error())
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
