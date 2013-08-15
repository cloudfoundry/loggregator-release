package logtarget

import (
	"github.com/stretchr/testify/assert"
	"net/url"
	"testing"
)

func TestIdentifier(t *testing.T) {
	target := &LogTarget{"org", "space", "app"}

	assert.Equal(t, target.Identifier(), "org:space:app")
}

var validityTests = []struct {
	org   string
	space string
	app   string
	valid bool
}{
	{"org", "space", "app", true},
	{"org", "space", "", true},
	{"org", "", "", true},
	{"org", "", "app", false},
	{"", "space", "app", false},
	{"", "space", "", false},
	{"", "", "app", false},
	{"", "", "", false},
}

func TestValidity(t *testing.T) {
	target := &LogTarget{}

	for _, validity := range validityTests {
		target.OrgId = validity.org
		target.SpaceId = validity.space
		target.AppId = validity.app
		assert.Equal(t, validity.valid, target.IsValid())
	}
}

func TestStringifying(t *testing.T) {
	target := LogTarget{"org", "space", "app"}
	expectedString := "Org: org, Space: space, App: app"

	assert.Equal(t, target.String(), expectedString)

}

func TestFromUrl(t *testing.T) {
	theUrl, err := url.Parse("wss://loggregator.loggregatorci.cf-app.com:4443/tail/?org=6e6926ce-bd94-428d-944f-9446ae446deb&space=e0c78fc4-443b-43d0-840f-ed8b0823b4fd&app=11bfecc7-7128-4e56-83a0-d8e0814ed7e6")
	assert.NoError(t, err)
	target := FromUrl(theUrl)
	assert.Equal(t, "6e6926ce-bd94-428d-944f-9446ae446deb", target.OrgId)
	assert.Equal(t, "e0c78fc4-443b-43d0-840f-ed8b0823b4fd", target.SpaceId)
	assert.Equal(t, "11bfecc7-7128-4e56-83a0-d8e0814ed7e6", target.AppId)
}
