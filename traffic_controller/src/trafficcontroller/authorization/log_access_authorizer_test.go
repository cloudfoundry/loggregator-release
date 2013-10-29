package authorization

import (
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"net/http"
	"regexp"
	"testing"
)

func init() {
	startFakeCloudController := func() {
		handleSpaceInfoRequest := func(w http.ResponseWriter, r *http.Request) {
			re := regexp.MustCompile("^/v2/apps/([^/?]+)$")
			result := re.FindStringSubmatch(r.URL.Path)
			if len(result) != 2 {
				w.WriteHeader(500)
				return
			}

			switch result[1] {
			case "myAppId":
				w.Write([]byte("{}"))
			case "notMyAppId":
				w.WriteHeader(403)
			default:
				w.WriteHeader(404)
			}
		}
		http.HandleFunc("/v2/apps/", handleSpaceInfoRequest)
		http.ListenAndServe(":9876", nil)
	}

	go startFakeCloudController()
}

var accessTests = []struct {
	target         string
	authToken      string
	expectedResult bool
}{
	//Allowed domains
	{
		"myAppId",
		"bearer something",
		true,
	},
	//Not allowed stuff
	{
		"notMyAppId",
		"bearer something",
		false,
	},
	{
		"nonExistantAppId",
		"bearer something",
		false,
	},
}

func TestUserRoleAccessCombinations(t *testing.T) {
	for i, test := range accessTests {
		authorizer := NewLogAccessAuthorizer("http://localhost:9876")
		result := authorizer(test.authToken, test.target, loggertesthelper.Logger())
		if result != test.expectedResult {
			t.Errorf("Access combination %d failed.", i)
		}
	}
}
