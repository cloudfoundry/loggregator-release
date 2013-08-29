package authorization

import (
	"github.com/cloudfoundry/loggregatorlib/logtarget"
	"github.com/cloudfoundry/loggregatorlib/testhelpers"
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
	userDetails    TokenPayload
	target         *logtarget.LogTarget
	authToken      string
	expectedResult bool
}{
	//Allowed domains
	{
		TokenPayload{UserId: "userId", Email: "user1@pivotallabs.com"},
		&logtarget.LogTarget{AppId: "myAppId"},
		"bearer something",
		true,
	},
	{
		TokenPayload{UserId: "userId", Email: "user2@gopivotal.com"},
		&logtarget.LogTarget{AppId: "myAppId"},
		"bearer something",
		true,
	},
	{
		TokenPayload{UserId: "userId", Email: "user3@vmware.com"},
		&logtarget.LogTarget{AppId: "myAppId"},
		"bearer something",
		true,
	},
	//Funky domain casing
	{
		TokenPayload{UserId: "userId", Email: "user3@VmWaRe.com"},
		&logtarget.LogTarget{AppId: "myAppId"},
		"bearer something",
		true,
	},
	//Not allowed stuff
	{
		TokenPayload{UserId: "userId", Email: "user3@vmware.com"},
		&logtarget.LogTarget{AppId: "notMyAppId"},
		"bearer something",
		false,
	},
	{
		TokenPayload{UserId: "userId", Email: "user3@vmware.com"},
		&logtarget.LogTarget{AppId: "nonExistantAppId"},
		"bearer something",
		false,
	},
	{
		TokenPayload{UserId: "userId", Email: "user3@gmail.com"},
		&logtarget.LogTarget{AppId: "myAppId"},
		"bearer something",
		false,
	},
}

func TestUserRoleAccessCombinations(t *testing.T) {
	for i, test := range accessTests {
		decoder := &TestUaaTokenDecoder{test.userDetails}
		authorizer := NewLogAccessAuthorizer(decoder, "http://localhost:9876")
		result := authorizer(test.authToken, test.target, testhelpers.Logger())
		if result != test.expectedResult {
			t.Errorf("Access combination %d for %v failed.", i, test.userDetails)
		}
	}
}
