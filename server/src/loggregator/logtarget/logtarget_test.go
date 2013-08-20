package logtarget

import (
	"github.com/stretchr/testify/assert"
	"net/url"
	"testing"
)

func TestIdentifier(t *testing.T) {
	target := &LogTarget{"app"}

	assert.Equal(t, target.Identifier(), "app")
}

var validityTests = []struct {
	app   string
	valid bool
}{
	{"app", true},
	{"", false},
}

func TestValidity(t *testing.T) {
	target := &LogTarget{}

	for _, validity := range validityTests {
		target.AppId = validity.app
		assert.Equal(t, validity.valid, target.IsValid())
	}
}

func TestStringifying(t *testing.T) {
	target := LogTarget{"app"}
	expectedString := "App: app"

	assert.Equal(t, target.String(), expectedString)

}

func TestFromUrl(t *testing.T) {
	theUrl, err := url.Parse("wss://loggregator.loggregatorci.cf-app.com:4443/tail/?app=11bfecc7-7128-4e56-83a0-d8e0814ed7e6")
	assert.NoError(t, err)
	target := FromUrl(theUrl)
	assert.Equal(t, "11bfecc7-7128-4e56-83a0-d8e0814ed7e6", target.AppId)
}
