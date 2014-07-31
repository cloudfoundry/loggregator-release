package main

import (
	"code.google.com/p/gogoprotobuf/proto"
	"flag"
	"fmt"
	"github.com/cloudfoundry/loggregatorlib/agentlistener"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
)

var host = flag.String("host", "0.0.0.0:3456", "The host/port you want to listen on")
var secret = flag.String("secret", "", "Our secret")

func main() {
	logger := loggertesthelper.Logger()

	flag.Parse()

	listener := agentlistener.NewAgentListener(*host, logger, "agentListener")
	dataChannel := listener.Start()

	for data := range dataChannel {
		if logMessage, err := extractLogMessage(data); err == nil {
			fmt.Printf("MESSAGE: %+v\n", logMessage)
		} else if logEnvelope, err := extractLogEnvelope(data); err == nil {
			validEnvelope := logEnvelope.VerifySignature(*secret)
			fmt.Printf("ENVELOPE (valid: %t): %+v\n", validEnvelope, logEnvelope)
		} else {
			fmt.Println("ERROR parsing protobuffer")
		}
	}
}

func extractLogMessage(data []byte) (*logmessage.LogMessage, error) {
	logMessage := new(logmessage.LogMessage)
	err := proto.Unmarshal(data, logMessage)
	if err != nil {
		return logMessage, err
	}
	return logMessage, nil
}

func extractLogEnvelope(data []byte) (*logmessage.LogEnvelope, error) {
	logEnvelope := new(logmessage.LogEnvelope)
	err := proto.Unmarshal(data, logEnvelope)
	if err != nil {
		return logEnvelope, err
	}
	return logEnvelope, nil
}
