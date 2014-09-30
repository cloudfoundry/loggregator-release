package authorization

import (
	"github.com/cloudfoundry/gosteno"
	"strings"
	"trafficcontroller/uaa_client"
)

const LOGGREGATOR_ADMIN_ROLE = "doppler.firehose"
const BEARER_PREFIX = "bearer "

type AdminAccessAuthorizer func(authToken string, logger *gosteno.Logger) bool

func NewAdminAccessAuthorizer(client uaa_client.UaaClient) AdminAccessAuthorizer {

	isAccessAllowed := func(authToken string, logger *gosteno.Logger) bool {
		authData, err := client.GetAuthData(strings.TrimPrefix(authToken, BEARER_PREFIX))

		if err != nil {
			logger.Errorf("Error getting auth data: %s", err.Error())
			return false
		}

		return authData.HasPermission(LOGGREGATOR_ADMIN_ROLE)
	}

	return AdminAccessAuthorizer(isAccessAllowed)
}
