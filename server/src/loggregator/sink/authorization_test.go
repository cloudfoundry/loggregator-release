package sink

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"regexp"
	"testhelpers"
	"testing"
)

func init() {
	startFakeCloudController := func() {
		handleSpaceInfoRequest := func(w http.ResponseWriter, r *http.Request) {
			re := regexp.MustCompile("^/v2/spaces/(.+)$")
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
				if depth != "1" {
					w.WriteHeader(404)
					return
				}
			}

			if result[1] == "send401Response" {
				w.WriteHeader(401)
				return
			}

			response :=
				`{
  "metadata": {
    "guid": "dcbb5518-533e-4d4c-a116-51fad762ddd4"
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
`
			w.Write([]byte(response))
		}

		http.HandleFunc("/v2/spaces/", handleSpaceInfoRequest)
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

func TestAllowsAccessForUserWhoIsSpaceManager(t *testing.T) {
	userDetails := map[string]interface{}{
		"user_id": "managerId",
	}

	decoder := &TestUaaTokenDecoder{userDetails}
	authorizer := NewLogAccessAuthorizer(decoder, "http://localhost:9876")
	result := authorizer("bearer manager", "mySpaceId", "myAppId", testhelpers.Logger())
	assert.True(t, result)
}

func TestAllowsAccessForUserWhoIsSpaceAuditor(t *testing.T) {
	userDetails := map[string]interface{}{
		"user_id": "auditorId",
	}

	decoder := &TestUaaTokenDecoder{userDetails}
	authorizer := NewLogAccessAuthorizer(decoder, "http://localhost:9876")
	result := authorizer("bearer auditor", "mySpaceId", "myAppId", testhelpers.Logger())
	assert.True(t, result)
}

func TestAllowsAccessForUserWhoIsSpaceDeveloper(t *testing.T) {
	userDetails := map[string]interface{}{
		"user_id": "developerId",
	}

	decoder := &TestUaaTokenDecoder{userDetails}
	authorizer := NewLogAccessAuthorizer(decoder, "http://localhost:9876")
	result := authorizer("bearer developer", "mySpaceId", "myAppId", testhelpers.Logger())
	assert.True(t, result)
}

func TestDeniesAccessForUserWhoIsNoneOfTheAbove(t *testing.T) {
	userDetails := map[string]interface{}{
		"user_id": "noneOfTheAboveId",
	}

	decoder := &TestUaaTokenDecoder{userDetails}
	authorizer := NewLogAccessAuthorizer(decoder, "http://localhost:9876")
	result := authorizer("bearer noneOfTheAbove", "mySpaceId", "myAppId", testhelpers.Logger())
	assert.False(t, result)
}

func TestDeniesAccessIfAppIsNotInSpace(t *testing.T) {
	userDetails := map[string]interface{}{
		"user_id": "developerId",
	}

	decoder := &TestUaaTokenDecoder{userDetails}
	authorizer := NewLogAccessAuthorizer(decoder, "http://localhost:9876")
	result := authorizer("bearer developer", "mySpaceId", "anotherAppId", testhelpers.Logger())
	assert.False(t, result)
}

func TestAllowsAccessIfYouDoNotHaveAppId(t *testing.T) {
	userDetails := map[string]interface{}{
		"user_id": "developerId",
	}

	decoder := &TestUaaTokenDecoder{userDetails}
	authorizer := NewLogAccessAuthorizer(decoder, "http://localhost:9876")
	result := authorizer("bearer developer", "mySpaceId", "", testhelpers.Logger())
	assert.True(t, result)
}

func TestDeniesAccessIfWeGetANon200Response(t *testing.T) {
	userDetails := map[string]interface{}{
		"user_id": "developerId",
	}

	decoder := &TestUaaTokenDecoder{userDetails}
	authorizer := NewLogAccessAuthorizer(decoder, "http://localhost:9876")
	result := authorizer("bearer developer", "send401Response", "", testhelpers.Logger())
	assert.False(t, result)
}
