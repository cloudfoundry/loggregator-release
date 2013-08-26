package authorization

import (
	"github.com/cloudfoundry/loggregatorlib/logtarget"
	"github.com/cloudfoundry/loggregatorlib/testhelpers"
	"github.com/stretchr/testify/assert"
	"net/http"
	"regexp"
	"testing"
)

func init() {
	startFakeCloudController := func() {
		handleSpaceInfoRequest := func(w http.ResponseWriter, r *http.Request) {
			re := regexp.MustCompile("^/v2/apps/(.+)$")
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
	{
		TokenPayload{UserId: "managerId", Scope: []string{"loggregator"}},
		&logtarget.LogTarget{AppId: "myAppId"},
		"bearer manager",
		true,
	},
	{
		TokenPayload{UserId: "managerId", Scope: []string{"notLoggregator"}},
		&logtarget.LogTarget{AppId: "myAppId"},
		"bearer manager",
		false,
	},
	{
		TokenPayload{UserId: "auditorId", Scope: []string{"loggregator"}},
		&logtarget.LogTarget{AppId: "myAppId"},
		"bearer auditor",
		true,
	},
	{
		TokenPayload{UserId: "auditorId", Scope: []string{"notLoggregator"}},
		&logtarget.LogTarget{AppId: "myAppId"},
		"bearer auditor",
		false,
	},
	{
		TokenPayload{UserId: "developerId", Scope: []string{"loggregator"}},
		&logtarget.LogTarget{AppId: "myAppId"},
		"bearer developer",
		true,
	},
	{
		TokenPayload{UserId: "developerId", Scope: []string{"notLoggregator"}},
		&logtarget.LogTarget{AppId: "myAppId"},
		"bearer developer",
		false,
	},
	{
		TokenPayload{UserId: "noneOfTheAboveId"},
		&logtarget.LogTarget{AppId: "myAppId"},
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

func TestDeniesAccessIfWeGetANon200Response(t *testing.T) {
	userDetails := TokenPayload{UserId: "developerId"}

	decoder := &TestUaaTokenDecoder{userDetails}
	target := &logtarget.LogTarget{
		AppId: "send401Response",
	}
	authorizer := NewLogAccessAuthorizer(decoder, "http://localhost:9876")
	result := authorizer("bearer developer", target, testhelpers.Logger())
	assert.False(t, result)
}

var response = `{
  "metadata": {
    "guid": "myAppId",
    "url": "/v2/apps/c4ed8615-4180-489e-9881-cf081a513f7f",
    "created_at": "2013-07-03 14:38:23 +0000",
    "updated_at": "2013-07-12 16:20:55 +0000"
  },
  "entity": {
    "name": "hello",
    "production": false,
    "space_guid": "3098caa9-63d2-4cfd-a9ed-e999981674de",
    "stack_guid": "8b233d46-b1f5-4069-8d45-c0a088b29237",
    "buildpack": null,
    "detected_buildpack": "Ruby/Rack",
    "environment_json": {
    },
    "memory": 64,
    "instances": 1,
    "disk_quota": 1024,
    "state": "STARTED",
    "version": "050303b7-a6fb-4d3f-983f-45ef1c9aae7f",
    "command": null,
    "console": true,
    "debug": null,
    "staging_task_id": "c1fc58c6d9076e111f594fc313157eeb",
    "space_url": "/v2/spaces/3098caa9-63d2-4cfd-a9ed-e999981674de",
    "space": {
      "metadata": {
        "guid": "3098caa9-63d2-4cfd-a9ed-e999981674de",
        "url": "/v2/spaces/3098caa9-63d2-4cfd-a9ed-e999981674de",
        "created_at": "2013-07-03 14:21:47 +0000",
        "updated_at": null
      },
      "entity": {
        "name": "arborglen",
        "organization_guid": "991816db-09bc-45cc-ad06-580ed84dc32c",
        "organization_url": "/v2/organizations/991816db-09bc-45cc-ad06-580ed84dc32c",
        "organization": {
          "metadata": {
            "guid": "991816db-09bc-45cc-ad06-580ed84dc32c",
            "url": "/v2/organizations/991816db-09bc-45cc-ad06-580ed84dc32c",
            "created_at": "2013-07-03 14:21:37 +0000",
            "updated_at": null
          },
          "entity": {
            "name": "arborglen",
            "billing_enabled": false,
            "quota_definition_guid": "901bc6ac-c888-4c8c-8ea0-e88d3c7fb505",
            "status": "active",
            "quota_definition_url": "/v2/quota_definitions/901bc6ac-c888-4c8c-8ea0-e88d3c7fb505",
            "spaces_url": "/v2/organizations/991816db-09bc-45cc-ad06-580ed84dc32c/spaces",
            "domains_url": "/v2/organizations/991816db-09bc-45cc-ad06-580ed84dc32c/domains",
            "users_url": "/v2/organizations/991816db-09bc-45cc-ad06-580ed84dc32c/users",
            "managers_url": "/v2/organizations/991816db-09bc-45cc-ad06-580ed84dc32c/managers",
            "billing_managers_url": "/v2/organizations/991816db-09bc-45cc-ad06-580ed84dc32c/billing_managers",
            "auditors_url": "/v2/organizations/991816db-09bc-45cc-ad06-580ed84dc32c/auditors",
            "app_events_url": "/v2/organizations/991816db-09bc-45cc-ad06-580ed84dc32c/app_events"
          }
        },
        "developers_url": "/v2/spaces/3098caa9-63d2-4cfd-a9ed-e999981674de/developers",
        "developers": [
          {
            "metadata": {
              "guid": "developerId",
              "url": "/v2/users/3b734e88-7256-4e02-b7fa-ff4c4f842497",
              "created_at": "2013-07-02 21:51:14 +0000",
              "updated_at": null
            },
            "entity": {
              "admin": true,
              "active": true,
              "default_space_guid": null,
              "spaces_url": "/v2/users/3b734e88-7256-4e02-b7fa-ff4c4f842497/spaces",
              "organizations_url": "/v2/users/3b734e88-7256-4e02-b7fa-ff4c4f842497/organizations",
              "managed_organizations_url": "/v2/users/3b734e88-7256-4e02-b7fa-ff4c4f842497/managed_organizations",
              "billing_managed_organizations_url": "/v2/users/3b734e88-7256-4e02-b7fa-ff4c4f842497/billing_managed_organizations",
              "audited_organizations_url": "/v2/users/3b734e88-7256-4e02-b7fa-ff4c4f842497/audited_organizations",
              "managed_spaces_url": "/v2/users/3b734e88-7256-4e02-b7fa-ff4c4f842497/managed_spaces",
              "audited_spaces_url": "/v2/users/3b734e88-7256-4e02-b7fa-ff4c4f842497/audited_spaces"
            }
          }
        ],
        "managers_url": "/v2/spaces/3098caa9-63d2-4cfd-a9ed-e999981674de/managers",
        "managers": [
          {
            "metadata": {
              "guid": "managerId",
              "url": "/v2/users/3b734e88-7256-4e02-b7fa-ff4c4f842497",
              "created_at": "2013-07-02 21:51:14 +0000",
              "updated_at": null
            },
            "entity": {
              "admin": true,
              "active": true,
              "default_space_guid": null,
              "spaces_url": "/v2/users/3b734e88-7256-4e02-b7fa-ff4c4f842497/spaces",
              "organizations_url": "/v2/users/3b734e88-7256-4e02-b7fa-ff4c4f842497/organizations",
              "managed_organizations_url": "/v2/users/3b734e88-7256-4e02-b7fa-ff4c4f842497/managed_organizations",
              "billing_managed_organizations_url": "/v2/users/3b734e88-7256-4e02-b7fa-ff4c4f842497/billing_managed_organizations",
              "audited_organizations_url": "/v2/users/3b734e88-7256-4e02-b7fa-ff4c4f842497/audited_organizations",
              "managed_spaces_url": "/v2/users/3b734e88-7256-4e02-b7fa-ff4c4f842497/managed_spaces",
              "audited_spaces_url": "/v2/users/3b734e88-7256-4e02-b7fa-ff4c4f842497/audited_spaces"
            }
          }
        ],
        "auditors_url": "/v2/spaces/3098caa9-63d2-4cfd-a9ed-e999981674de/auditors",
        "auditors": [
			{
            "metadata": {
              "guid": "auditorId",
              "url": "/v2/users/3b734e88-7256-4e02-b7fa-ff4c4f842497",
              "created_at": "2013-07-02 21:51:14 +0000",
              "updated_at": null
            },
            "entity": {
              "admin": true,
              "active": true,
              "default_space_guid": null,
              "spaces_url": "/v2/users/3b734e88-7256-4e02-b7fa-ff4c4f842497/spaces",
              "organizations_url": "/v2/users/3b734e88-7256-4e02-b7fa-ff4c4f842497/organizations",
              "managed_organizations_url": "/v2/users/3b734e88-7256-4e02-b7fa-ff4c4f842497/managed_organizations",
              "billing_managed_organizations_url": "/v2/users/3b734e88-7256-4e02-b7fa-ff4c4f842497/billing_managed_organizations",
              "audited_organizations_url": "/v2/users/3b734e88-7256-4e02-b7fa-ff4c4f842497/audited_organizations",
              "managed_spaces_url": "/v2/users/3b734e88-7256-4e02-b7fa-ff4c4f842497/managed_spaces",
              "audited_spaces_url": "/v2/users/3b734e88-7256-4e02-b7fa-ff4c4f842497/audited_spaces"
            }
          }
        ],
        "apps_url": "/v2/spaces/3098caa9-63d2-4cfd-a9ed-e999981674de/apps",
        "domains_url": "/v2/spaces/3098caa9-63d2-4cfd-a9ed-e999981674de/domains",
        "domains": [
          {
            "metadata": {
              "guid": "cfb88df5-7146-44b3-8d61-5cce7d2fe505",
              "url": "/v2/domains/cfb88df5-7146-44b3-8d61-5cce7d2fe505",
              "created_at": "2013-07-02 21:42:08 +0000",
              "updated_at": null
            },
            "entity": {
              "name": "arborglen.cf-app.com",
              "owning_organization_guid": null,
              "wildcard": true,
              "spaces_url": "/v2/domains/cfb88df5-7146-44b3-8d61-5cce7d2fe505/spaces"
            }
          }
        ],
        "service_instances_url": "/v2/spaces/3098caa9-63d2-4cfd-a9ed-e999981674de/service_instances",
        "service_instances": [

        ],
        "app_events_url": "/v2/spaces/3098caa9-63d2-4cfd-a9ed-e999981674de/app_events",
        "events_url": "/v2/spaces/3098caa9-63d2-4cfd-a9ed-e999981674de/events",
        "events": [

        ]
      }
    },
    "stack_url": "/v2/stacks/8b233d46-b1f5-4069-8d45-c0a088b29237",
    "stack": {
      "metadata": {
        "guid": "8b233d46-b1f5-4069-8d45-c0a088b29237",
        "url": "/v2/stacks/8b233d46-b1f5-4069-8d45-c0a088b29237",
        "created_at": "2013-08-13 22:29:04 +0000",
        "updated_at": "2013-08-13 22:29:04 +0000"
      },
      "entity": {
        "name": "lucid64",
        "description": "Ubuntu 10.04"
      }
    },
    "service_bindings_url": "/v2/apps/c4ed8615-4180-489e-9881-cf081a513f7f/service_bindings",
    "service_bindings": [

    ],
    "routes_url": "/v2/apps/c4ed8615-4180-489e-9881-cf081a513f7f/routes",
    "routes": [
      {
        "metadata": {
          "guid": "c5f8c5cd-05e5-4118-9f0c-a7f6e613a5a3",
          "url": "/v2/routes/c5f8c5cd-05e5-4118-9f0c-a7f6e613a5a3",
          "created_at": "2013-07-03 14:38:27 +0000",
          "updated_at": null
        },
        "entity": {
          "host": "hello",
          "domain_guid": "cfb88df5-7146-44b3-8d61-5cce7d2fe505",
          "space_guid": "3098caa9-63d2-4cfd-a9ed-e999981674de",
          "domain_url": "/v2/domains/cfb88df5-7146-44b3-8d61-5cce7d2fe505",
          "domain": {
            "metadata": {
              "guid": "cfb88df5-7146-44b3-8d61-5cce7d2fe505",
              "url": "/v2/domains/cfb88df5-7146-44b3-8d61-5cce7d2fe505",
              "created_at": "2013-07-02 21:42:08 +0000",
              "updated_at": null
            },
            "entity": {
              "name": "arborglen.cf-app.com",
              "owning_organization_guid": null,
              "wildcard": true,
              "spaces_url": "/v2/domains/cfb88df5-7146-44b3-8d61-5cce7d2fe505/spaces"
            }
          },
          "space_url": "/v2/spaces/3098caa9-63d2-4cfd-a9ed-e999981674de",
          "space": {
            "metadata": {
              "guid": "3098caa9-63d2-4cfd-a9ed-e999981674de",
              "url": "/v2/spaces/3098caa9-63d2-4cfd-a9ed-e999981674de",
              "created_at": "2013-07-03 14:21:47 +0000",
              "updated_at": null
            },
            "entity": {
              "name": "arborglen",
              "organization_guid": "991816db-09bc-45cc-ad06-580ed84dc32c",
              "organization_url": "/v2/organizations/991816db-09bc-45cc-ad06-580ed84dc32c",
              "developers_url": "/v2/spaces/3098caa9-63d2-4cfd-a9ed-e999981674de/developers",
              "managers_url": "/v2/spaces/3098caa9-63d2-4cfd-a9ed-e999981674de/managers",
              "auditors_url": "/v2/spaces/3098caa9-63d2-4cfd-a9ed-e999981674de/auditors",
              "apps_url": "/v2/spaces/3098caa9-63d2-4cfd-a9ed-e999981674de/apps",
              "domains_url": "/v2/spaces/3098caa9-63d2-4cfd-a9ed-e999981674de/domains",
              "service_instances_url": "/v2/spaces/3098caa9-63d2-4cfd-a9ed-e999981674de/service_instances",
              "app_events_url": "/v2/spaces/3098caa9-63d2-4cfd-a9ed-e999981674de/app_events",
              "events_url": "/v2/spaces/3098caa9-63d2-4cfd-a9ed-e999981674de/events"
            }
          },
          "apps_url": "/v2/routes/c5f8c5cd-05e5-4118-9f0c-a7f6e613a5a3/apps"
        }
      }
    ],
    "events_url": "/v2/apps/c4ed8615-4180-489e-9881-cf081a513f7f/events"
  }
}`
