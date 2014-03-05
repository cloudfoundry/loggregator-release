package unmarshaller_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.google.com/p/gogoprotobuf/proto"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"testing"
	"time"
)

func MarshalledLogEnvelope(unmarshalledMessage *logmessage.LogMessage, secret string) []byte {
	envelope := &logmessage.LogEnvelope{
		LogMessage: unmarshalledMessage,
		RoutingKey: proto.String(*unmarshalledMessage.AppId),
	}
	envelope.SignEnvelope(secret)

	return marshalProtoBuf(envelope)
}

func GenerateLogMessage(messageString, appId string, messageType logmessage.LogMessage_MessageType, sourceName, sourceId string) *logmessage.LogMessage {
	currentTime := time.Now()
	logMessage := &logmessage.LogMessage{
		Message:     []byte(messageString),
		AppId:       proto.String(appId),
		MessageType: &messageType,
		SourceName:  proto.String(sourceName),
		SourceId:    proto.String(sourceId),
		Timestamp:   proto.Int64(currentTime.UnixNano()),
	}

	return logMessage
}

func marshalProtoBuf(pb proto.Message) []byte {
	marshalledProtoBuf, err := proto.Marshal(pb)
	if err != nil {
		Fail(err.Error())
	}

	return marshalledProtoBuf
}

func TestUnmarshaller(t *testing.T) {
	cfcomponent.Logger = loggertesthelper.Logger()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Unmarshaller Suite")
}
