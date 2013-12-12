package deaagent

import (
	"deaagent/metadataservice"
	"deaagent_testhelpers"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestThatItEmitsToLoggregator(t *testing.T) {

	mockEmitter := &deaagent_testhelpers.MockLoggregatorEmitter{
		Received: make(chan *logmessage.LogMessage),
	}

	inputChan := make(chan []byte, 5)
	data := make(map[string]metadataservice.Metadata)
	data["warden_handle"] = metadataservice.Metadata{
		Index:           "42",
		Guid:            "some-app-id",
		SyslogDrainUrls: []string{"drain-uri-1", "drain-uri-2"},
	}
	metaDataService := deaagent_testhelpers.FakeMetaDataService{Data: data}
	agent := Agent{deaagent_testhelpers.FakeListener{SendingChan: inputChan}, metaDataService, mockEmitter, loggertesthelper.Logger()}

	agent.Start()

	inputChan <- []byte("<14>2013-12-11T13:50:09-08:00 majestic warden.App.warden_handle[123]: TEST MESSAGE\n")

	select {
	case message := <-mockEmitter.Received:
		assert.Equal(t, message.GetAppId(), "some-app-id")
		assert.Equal(t, message.GetDrainUrls(), []string{"drain-uri-1", "drain-uri-2"})
		assert.Equal(t, string(message.GetMessage()), "TEST MESSAGE")
		assert.Equal(t, message.GetSourceId(), "42")
		assert.Equal(t, message.GetMessageType(), logmessage.LogMessage_OUT)
		assert.Equal(t, message.GetSourceName(), "App")
		assert.Equal(t, message.GetTimestamp(), 1386798609000000000)
	case <-time.After(5 * time.Second):
		t.Errorf("Timed out waiting for message")
	}
}
