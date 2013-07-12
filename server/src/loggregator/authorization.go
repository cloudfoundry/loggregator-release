package loggregator

import (
	"github.com/cloudfoundry/gosteno"

//	"net/http"
)

type LogAccessAuthorizer func(string, string, string, *gosteno.Logger) bool

func AuthorizedToListenToLogs(apiHost, authToken, spaceId string, logger *gosteno.Logger) bool {
	//	client := &http.Client{}
	//
	//	req, _ := http.NewRequest("GET", apiHost+"/v2/apps/"+appId, nil)
	//	req.Header.Set("Authorization", authToken)
	//	res, err := client.Do(req)
	//
	//	if err != nil {
	//		logger.Errorf("Did not accept sink connection because of connection issue with CC: %v", err)
	//		return false
	//	}
	//	if res.StatusCode != 200 {
	//		return false
	//	}

	return true
}
