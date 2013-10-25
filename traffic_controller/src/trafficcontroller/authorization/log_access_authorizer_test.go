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

type TestUaaTokenDecoder struct {
	details TokenPayload
}

func (d TestUaaTokenDecoder) Decode(token string) (TokenPayload, error) {
	return d.details, nil
}

var accessTests = []struct {
	userDetails                     TokenPayload
	target                          string
	authToken                       string
	disableEmailDomainAuthorization bool
	expectedResult                  bool
}{
	//Allowed domains
	{
		TokenPayload{UserId: "userId", Email: "user1@pivotallabs.com"},
		"myAppId",
		"bearer something",
		false,
		true,
	},
	{
		TokenPayload{UserId: "userId", Email: "user2@gopivotal.com"},
		"myAppId",
		"bearer something",
		false,
		true,
	},
	{
		TokenPayload{UserId: "userId", Email: "user3@vmware.com"},
		"myAppId",
		"bearer something",
		false,
		true,
	},
	//Funky domain casing
	{
		TokenPayload{UserId: "userId", Email: "user3@VmWaRe.com"},
		"myAppId",
		"bearer something",
		false,
		true,
	},
	//Allowed domains - disabled email domain authorization
	{
		TokenPayload{UserId: "userId", Email: "user3@gmail.com"},
		"myAppId",
		"bearer something",
		true,
		true,
	},
	//Not allowed stuff
	{
		TokenPayload{UserId: "userId", Email: "user3@vmware.com"},
		"notMyAppId",
		"bearer something",
		false,
		false,
	},
	{
		TokenPayload{UserId: "userId", Email: "user3@vmware.com"},
		"nonExistantAppId",
		"bearer something",
		false,
		false,
	},
	{
		TokenPayload{UserId: "userId", Email: "user3@gmail.com"},
		"myAppId",
		"bearer something",
		false,
		false,
	},
}

func TestUserRoleAccessCombinations(t *testing.T) {
	for i, test := range accessTests {
		decoder := &TestUaaTokenDecoder{test.userDetails}
		authorizer := NewLogAccessAuthorizer(decoder, "http://localhost:9876", test.disableEmailDomainAuthorization)
		result := authorizer(test.authToken, test.target, loggertesthelper.Logger())
		if result != test.expectedResult {
			t.Errorf("Access combination %d for %v failed.", i, test.userDetails)
		}
	}
}
