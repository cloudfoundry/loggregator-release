package sink

import (
	"github.com/stretchr/testify/assert"
	"loggregator/logtarget"
	"net/http"
	"regexp"
	"testhelpers"
	"testing"
)

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

			response := `
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
			w.Write([]byte(response))
		}

		http.HandleFunc("/v2/organizations/", handleSpaceInfoRequest)
		http.ListenAndServe(":9876", nil)
	}

	go startFakeCloudController()
}

type TestUaaTokenDecoder struct {
	details map[string]interface{}
}

func (d TestUaaTokenDecoder) Decode(token string) (map[string]interface{}, error) {
	return d.details, nil
}

func TestAllowsAccessForUserWhoIsOrgManager(t *testing.T) {
	userDetails := map[string]interface{}{
		"user_id": "orgManagerId",
	}

	decoder := &TestUaaTokenDecoder{userDetails}
	authorizer := NewLogAccessAuthorizer(decoder, "http://localhost:9876")
	target := &logtarget.LogTarget{
		OrgId: "myOrgId",
	}
	result := authorizer("bearer orgManagerId", target, testhelpers.Logger())
	assert.True(t, result)
}

func TestAllowsAccessForUserWhoIsOrgAuditor(t *testing.T) {
	userDetails := map[string]interface{}{
		"user_id": "orgAuditorId",
	}

	decoder := &TestUaaTokenDecoder{userDetails}
	authorizer := NewLogAccessAuthorizer(decoder, "http://localhost:9876")
	target := &logtarget.LogTarget{
		OrgId: "myOrgId",
	}
	result := authorizer("bearer orgAuditor", target, testhelpers.Logger())
	assert.True(t, result)
}

func TestAllowsAccessForUserWhoIsSpaceManager(t *testing.T) {
	userDetails := map[string]interface{}{
		"user_id": "managerId",
	}

	decoder := &TestUaaTokenDecoder{userDetails}
	authorizer := NewLogAccessAuthorizer(decoder, "http://localhost:9876")
	target := &logtarget.LogTarget{
		OrgId:   "myOrgId",
		SpaceId: "mySpaceId",
		AppId:   "myAppId",
	}
	result := authorizer("bearer manager", target, testhelpers.Logger())
	assert.True(t, result)
}

func TestAllowsAccessForUserWhoIsSpaceAuditor(t *testing.T) {
	userDetails := map[string]interface{}{
		"user_id": "auditorId",
	}

	decoder := &TestUaaTokenDecoder{userDetails}
	target := &logtarget.LogTarget{
		OrgId:   "myOrgId",
		SpaceId: "mySpaceId",
		AppId:   "myAppId",
	}
	authorizer := NewLogAccessAuthorizer(decoder, "http://localhost:9876")
	result := authorizer("bearer auditor", target, testhelpers.Logger())
	assert.True(t, result)
}

func TestAllowsAccessForUserWhoIsSpaceDeveloper(t *testing.T) {
	userDetails := map[string]interface{}{
		"user_id": "developerId",
	}

	decoder := &TestUaaTokenDecoder{userDetails}
	target := &logtarget.LogTarget{
		OrgId:   "myOrgId",
		SpaceId: "mySpaceId",
		AppId:   "myAppId",
	}
	authorizer := NewLogAccessAuthorizer(decoder, "http://localhost:9876")
	result := authorizer("bearer developer", target, testhelpers.Logger())
	assert.True(t, result)
}

func TestDeniesAccessForUserWhoIsNoneOfTheAbove(t *testing.T) {
	userDetails := map[string]interface{}{
		"user_id": "noneOfTheAboveId",
	}

	decoder := &TestUaaTokenDecoder{userDetails}
	target := &logtarget.LogTarget{
		OrgId:   "myOrgId",
		SpaceId: "mySpaceId",
		AppId:   "myAppId",
	}
	authorizer := NewLogAccessAuthorizer(decoder, "http://localhost:9876")
	result := authorizer("bearer noneOfTheAbove", target, testhelpers.Logger())
	assert.False(t, result)
}

func TestDeniesAccessIfSpaceIsNotInOrg(t *testing.T) {
	userDetails := map[string]interface{}{
		"user_id": "developerId",
	}

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
	userDetails := map[string]interface{}{
		"user_id": "developerId",
	}

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
	userDetails := map[string]interface{}{
		"user_id": "developerId",
	}

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
	userDetails := map[string]interface{}{
		"user_id": "developerId",
	}

	decoder := &TestUaaTokenDecoder{userDetails}
	target := &logtarget.LogTarget{
		OrgId: "send401Response",
	}
	authorizer := NewLogAccessAuthorizer(decoder, "http://localhost:9876")
	result := authorizer("bearer developer", target, testhelpers.Logger())
	assert.False(t, result)
}
