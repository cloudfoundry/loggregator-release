package syslogreader

import (
	"testing"

	"deaagent/metadataservice"
	"deaagent_testhelpers"
	"errors"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/stretchr/testify/assert"
	"io"
	"time"
)

var metaDataService deaagent_testhelpers.FakeMetaDataService

func init() {
	metaData := make(map[string]metadataservice.Metadata)
	metaData["warden_handle"] = metadataservice.Metadata{
		Index:           "42",
		Guid:            "some-app-id",
		SyslogDrainUrls: []string{"drain-uri-1", "drain-uri-2"},
	}
	metaDataService = deaagent_testhelpers.FakeMetaDataService{Data: metaData}
}

func TestThatItCanReadSyslog(t *testing.T) {
	rawMessage := []byte("<14>2013-12-11T13:50:09-08:00 majestic warden.App.warden_handle[75070]: TEST MESSAGE\n")

	message, err := ReadMessage(metaDataService, rawMessage)
	assert.NoError(t, err)

	assert.Equal(t, message.GetAppId(), "some-app-id")
	assert.Equal(t, message.GetDrainUrls(), []string{"drain-uri-1", "drain-uri-2"})
	assert.Equal(t, string(message.GetMessage()), "TEST MESSAGE")
	assert.Equal(t, message.GetSourceId(), "42")
	assert.Equal(t, message.GetMessageType(), logmessage.LogMessage_OUT)
	assert.Equal(t, message.GetSourceName(), "App")
	assert.Equal(t, message.GetTimestamp(), 1386798609000000000)
}

func TestReturnsEOFWhenMessageInterrupted(t *testing.T) {
	messageWithTerminatedPriority := []byte("<6")

	_, err := ReadMessage(metaDataService, messageWithTerminatedPriority)
	assert.Equal(t, io.EOF, err)

	messageWithTerminatedTimestamp := []byte("<6>2013-12-11T13:50")
	_, err = ReadMessage(metaDataService, messageWithTerminatedTimestamp)
	assert.Equal(t, io.EOF, err)

	messageWithTerminatedHostname := []byte("<6>2013-12-11T13:50:09-08:00 majestic")
	_, err = ReadMessage(metaDataService, messageWithTerminatedHostname)
	assert.Equal(t, io.EOF, err)

	messageWithTerminatedTag := []byte("<6>2013-12-11T13:50:09-08:00 majestic syslog_test")
	_, err = ReadMessage(metaDataService, messageWithTerminatedTag)
	assert.Equal(t, io.EOF, err)

	messageWithTerminatedPid := []byte("<6>2013-12-11T13:50:09-08:00 majestic syslog_test[123")
	_, err = ReadMessage(metaDataService, messageWithTerminatedPid)
	assert.Equal(t, io.EOF, err)
}

func TestMessageDoesNotEndWithNewline(t *testing.T) {
	rawMessage := []byte("<14>2013-12-11T13:50:09-08:00 majestic warden.App.warden_handle[75070]: TEST MESSAGE\n")

	message, _ := ReadMessage(metaDataService, rawMessage)
	assert.Equal(t, "TEST MESSAGE", string(message.GetMessage()))

	rawMessage = []byte("<14>2013-12-11T13:50:09-08:00 majestic warden.App.warden_handle[75070]: TEST MESSAGE")

	message, _ = ReadMessage(metaDataService, rawMessage)
	assert.Equal(t, "TEST MESSAGE", string(message.GetMessage()))
}

func TestMessageWithInvalidTimestamp(t *testing.T) {
	rawMessage := []byte("<14>2013-12-11T13:5009-08:00 majestic warden.App.warden_handle[75070]: TEST MESSAGE\n")

	_, err := ReadMessage(metaDataService, rawMessage)
	assert.IsType(t, &time.ParseError{}, err)
}

func TestMessageWithInvalidTags(t *testing.T) {
	rawMessage := []byte("<14>2013-12-11T13:50:09-08:00 majestic warden.BADTAG[75070]: TEST MESSAGE\n")

	_, err := ReadMessage(metaDataService, rawMessage)
	assert.Equal(t, errors.New("Invalid Tags"), err)

	rawMessage = []byte("<14>2013-12-11T13:50:09-08:00 majestic warden.BADTAG.warden_handle.extra_tag[75070]: TEST MESSAGE\n")
	_, err = ReadMessage(metaDataService, rawMessage)
	assert.Equal(t, errors.New("Invalid Tags"), err)

	rawMessage = []byte("<14>2013-12-11T13:50:09-08:00 majestic [75070]: TEST MESSAGE\n")
	_, err = ReadMessage(metaDataService, rawMessage)
	assert.Equal(t, errors.New("Invalid Tags"), err)
}

func TestMessageWithInvalidPriority(t *testing.T) {
	rawMessage := []byte("<171>2013-12-11T13:50:09-08:00 majestic warden.App.warden_handle[75070]: TEST MESSAGE\n")
	_, err := ReadMessage(metaDataService, rawMessage)
	assert.Equal(t, errors.New("Invalid Priority"), err)

	rawMessage = []byte("<ERROR>2013-12-11T13:50:09-08:00 majestic warden.App.warden_handle[75070]: TEST MESSAGE\n")
	_, err = ReadMessage(metaDataService, rawMessage)
	assert.Equal(t, errors.New("Invalid Priority"), err)

	rawMessage = []byte("<>2013-12-11T13:50:09-08:00 majestic warden.App.warden_handle[75070]: TEST MESSAGE\n")
	_, err = ReadMessage(metaDataService, rawMessage)
	assert.Equal(t, errors.New("Invalid Priority"), err)
}

func TestMessageWithInvalidWardenHandle(t *testing.T) {
	rawMessage := []byte("<14>2013-12-11T13:50:09-08:00 majestic warden.App.Invalid_warden_handle[75070]: TEST MESSAGE\n")
	_, err := ReadMessage(metaDataService, rawMessage)
	assert.Equal(t, errors.New("Not Found"), err)
}
