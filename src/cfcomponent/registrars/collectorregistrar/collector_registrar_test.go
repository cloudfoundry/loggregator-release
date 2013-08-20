package collectorregistrar

import (
	"cfcomponent"
	"encoding/json"
	"github.com/cloudfoundry/go_cfmessagebus/mock_cfmessagebus"
	"github.com/stretchr/testify/assert"
	"testhelpers"
	"testing"
)

func TestAnnounceComponent(t *testing.T) {
	cfc := cfcomponent.Component{
		IpAddress:         "1.2.3.4",
		Type:              "Loggregator Server",
		Index:             0,
		StatusPort:        5678,
		StatusCredentials: []string{"user", "pass"},
		UUID:              "abc123",
	}
	mbus := mock_cfmessagebus.NewMockMessageBus()

	called := make(chan []byte)
	callback := func(response []byte) {
		called <- response
	}
	mbus.Subscribe(AnnounceComponentMessageSubject, callback)

	registrar := NewCollectorRegistrar(mbus, testhelpers.Logger())

	registrar.announceComponent(cfc)

	actual := <-called

	expected := &AnnounceComponentMessage{
		Type:        "Loggregator Server",
		Index:       0,
		Host:        "1.2.3.4:5678",
		UUID:        "0-abc123",
		Credentials: []string{"user", "pass"},
	}

	expectedJson, err := json.Marshal(expected)
	assert.NoError(t, err)
	assert.Equal(t, string(expectedJson), string(actual))
}

func TestSubscribeToComponentDiscover(t *testing.T) {
	cfc := cfcomponent.Component{
		IpAddress:         "1.2.3.4",
		Type:              "Loggregator Server",
		Index:             0,
		StatusPort:        5678,
		StatusCredentials: []string{"user", "pass"},
		UUID:              "abc123",
	}

	mbus := mock_cfmessagebus.NewMockMessageBus()
	registrar := NewCollectorRegistrar(mbus, testhelpers.Logger())

	registrar.subscribeToComponentDiscover(cfc)

	called := make(chan []byte)
	callback := func(response []byte) {
		called <- response
	}

	message := []byte("")
	mbus.Request(DiscoverComponentMessageSubject, message, callback)

	expected := &AnnounceComponentMessage{
		Type:        "Loggregator Server",
		Index:       0,
		Host:        "1.2.3.4:5678",
		UUID:        "0-abc123",
		Credentials: []string{"user", "pass"},
	}

	expectedJson, err := json.Marshal(expected)
	assert.NoError(t, err)
	assert.Equal(t, string(expectedJson), string(<-called))
}
