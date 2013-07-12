package loggregator

import (
	//	"github.com/stretchr/testify/assert"
	"net/http"

//	"testing"
)

func init() {
	startFakeCloudController := func() {
		handleCloudControllerRequest := func(w http.ResponseWriter, r *http.Request) {
			if r.Header["Authorization"][0] != "bearer correctAuthorizationToken" {
				w.WriteHeader(401)
			}
		}
		http.HandleFunc("/v2/apps/", handleCloudControllerRequest)
		http.ListenAndServe(":9876", nil)
	}

	go startFakeCloudController()
}

//func TestAllowsAccessForUserWhoIsSpaceManager(t *testing.T) {
//	result := AuthorizedToListenToLogs("http://localhost:9876", logger(), "bearer correctAuthorizationToken", "myAppId", "myUserId")
//	assert.True(t, result)
//}

//func TestAllowsAccessForUserWhoIsSpaceAuditor(t *testing.T) {
//
//}
//
//func TestAllowsAccessForUserWhoIsSpaceDeveloper(t *testing.T) {
//
//}
//
//fund TestDeniesAccessForUserWhoIsNoneOfTheAbove(t *testing.T) {
//
//}

//func TestAllowsAccessForAValidToken(t *testing.T) {
//	result := AuthorizedToListenToLogs("http://localhost:9876", logger(), "bearer correctAuthorizationToken", "myAppId")
//	assert.True(t, result)
//}
//
//func TestDenysAccessForAnInvalidToken(t *testing.T) {
//	result := AuthorizedToListenToLogs("http://localhost:9876", logger(), "bearer incorrectAuthorizationToken", "myAppId")
//	assert.False(t, result)
//}
