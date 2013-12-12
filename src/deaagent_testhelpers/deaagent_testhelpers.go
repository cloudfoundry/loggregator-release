package deaagent_testhelpers

import (
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"deaagent/metadataservice"
	"errors"
)

type MockLoggregatorEmitter struct {
	Received chan *logmessage.LogMessage
}

func (m MockLoggregatorEmitter) Emit(a, b string) {

}

func (m MockLoggregatorEmitter) EmitError(a, b string) {

}

func (m MockLoggregatorEmitter) EmitLogMessage(message *logmessage.LogMessage) {
	m.Received <- message
}

type FakeMetaDataService struct {
	Data map[string]metadataservice.Metadata
}

func (s FakeMetaDataService) Lookup(wardenHandle string) (metadataservice.Metadata, error) {
	metadata := s.Data[wardenHandle]
	if metadata.Guid == "" {
		return metadata, errors.New("Not Found")
	}
	return metadata, nil
}

type FakeListener struct {
	SendingChan chan []byte
}

func (l FakeListener) Start() chan []byte {
	return l.SendingChan
}

func (l FakeListener) Emit() instrumentation.Context {
	return instrumentation.Context{}
}
