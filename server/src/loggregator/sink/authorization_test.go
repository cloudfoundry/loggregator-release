package sink

import (
	"github.com/stretchr/testify/assert"
	"loggregator/logtarget"
	"net/http"
	"regexp"
	"testhelpers"
	"testing"
)

var response = `
        {
            "metadata": {
                "guid": "myOrgId"
            },
            "entity": {
                "name": "MyOrg",
                "status": "active",
                "spaces": [
                    {
                        "metadata": {
                            "guid": "mySpaceId"
                        },
                        "entity": {
                            "name": "Dev1",
                            "developers": [
                                {
                                    "metadata": {
                                        "guid": "developerId"
                                    }
                                }
                            ],
                            "managers": [
                                {
                                    "metadata": {
                                        "guid": "managerId"
                                    }
                                }
                            ],
                            "auditors": [
                                {
                                    "metadata": {
                                        "guid": "auditorId"
                                    }
                                }
                            ],
                            "apps": [
                                {
                                    "metadata": {
                                        "guid": "myAppId"
                                    }
                                }
                            ]
                        }
                    }
                ],
                "managers": [
                    {
                        "metadata": {
                            "guid": "orgManagerId"
                        }
                    }
                ],
                "auditors": [
                    {
                        "metadata": {
                            "guid": "orgAuditorId"
                        }
                    }
                ]
            }
        }`

func init() {
	startFakeCloudController := func() {
		handleSpaceInfoRequest := func(w http.ResponseWriter, r *http.Request) {
			re := regexp.MustCompile("^/v2/organizations/(.+)$")
			result := re.FindStringSubmatch(r.URL.Path)
			if len(result) != 2 {
				w.WriteHeader(404)
				return
			}

			depth := ""
			queryValues := r.URL.Query()
			if len(queryValues["inline-relations-depth"]) != 1 {
				w.WriteHeader(404)
				return
			} else {
				depth = queryValues["inline-relations-depth"][0]
				if depth != "2" {
					w.WriteHeader(404)
					return
				}
			}

			if result[1] == "send401Response" {
				w.WriteHeader(401)
				return
			}
			w.Write([]byte(response))
		}
		http.HandleFunc("/v2/organizations/", handleSpaceInfoRequest)
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
	{
		TokenPayload{UserId: "orgManagerId"},
		&logtarget.LogTarget{OrgId: "myOrgId"},
		"bearer orgManagerId",
		true,
	},
	{
		TokenPayload{UserId: "orgAuditorId"},
		&logtarget.LogTarget{OrgId: "myOrgId"},
		"bearer orgAuditor",
		true,
	},
	{
		TokenPayload{UserId: "managerId"},
		&logtarget.LogTarget{OrgId: "myOrgId", SpaceId: "mySpaceId", AppId: "myAppId"},
		"bearer manager",
		true,
	},
	{
		TokenPayload{UserId: "auditorId"},
		&logtarget.LogTarget{OrgId: "myOrgId", SpaceId: "mySpaceId", AppId: "myAppId"},
		"bearer auditor",
		true,
	},
	{
		TokenPayload{UserId: "developerId"},
		&logtarget.LogTarget{OrgId: "myOrgId", SpaceId: "mySpaceId", AppId: "myAppId"},
		"bearer developer",
		true,
	},
	{
		TokenPayload{UserId: "noneOfTheAboveId"},
		&logtarget.LogTarget{OrgId: "myOrgId", SpaceId: "mySpaceId", AppId: "myAppId"},
		"bearer noneOfTheAbove",
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

func TestDeniesAccessIfSpaceIsNotInOrg(t *testing.T) {
	userDetails := TokenPayload{UserId: "developerId"}

	decoder := &TestUaaTokenDecoder{userDetails}
	target := &logtarget.LogTarget{
		OrgId:   "myOrgId",
		SpaceId: "anotherSpaceId",
		AppId:   "myAppId",
	}
	authorizer := NewLogAccessAuthorizer(decoder, "http://localhost:9876")
	result := authorizer("bearer developer", target, testhelpers.Logger())
	assert.False(t, result)
}

func TestDeniesAccessIfAppIsNotInSpace(t *testing.T) {
	userDetails := TokenPayload{UserId: "developerId"}

	decoder := &TestUaaTokenDecoder{userDetails}
	target := &logtarget.LogTarget{
		OrgId:   "myOrgId",
		SpaceId: "mySpaceId",
		AppId:   "anotherAppId",
	}
	authorizer := NewLogAccessAuthorizer(decoder, "http://localhost:9876")
	result := authorizer("bearer developer", target, testhelpers.Logger())
	assert.False(t, result)
}

func TestAllowsAccessIfYouDoNotHaveAppId(t *testing.T) {
	userDetails := TokenPayload{UserId: "developerId"}

	decoder := &TestUaaTokenDecoder{userDetails}
	target := &logtarget.LogTarget{
		OrgId:   "myOrgId",
		SpaceId: "mySpaceId",
	}
	authorizer := NewLogAccessAuthorizer(decoder, "http://localhost:9876")
	result := authorizer("bearer developer", target, testhelpers.Logger())
	assert.True(t, result)
}

func TestDeniesAccessIfWeGetANon200Response(t *testing.T) {
	userDetails := TokenPayload{UserId: "developerId"}

	decoder := &TestUaaTokenDecoder{userDetails}
	target := &logtarget.LogTarget{
		OrgId: "send401Response",
	}
	authorizer := NewLogAccessAuthorizer(decoder, "http://localhost:9876")
	result := authorizer("bearer developer", target, testhelpers.Logger())
	assert.False(t, result)
}
