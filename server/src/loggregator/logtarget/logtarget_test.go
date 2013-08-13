package logtarget

import (
	"testing"
	"github.com/stretchr/testify/assert"
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
