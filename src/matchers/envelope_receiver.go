package matchers

import (
	"errors"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	"github.com/onsi/gomega/types"
)

type envelopeReceiver struct {
	matcher        types.GomegaMatcher
	foundEnvelope  *events.Envelope
	mustHavePrefix bool
	hadPrefix      bool
}

func ReceivePrefixedEnvelope(matchers ...types.GomegaMatcher) types.GomegaMatcher {
	return receiveEnvelope(true, matchers...)
}

func ReceiveEnvelope(matchers ...types.GomegaMatcher) types.GomegaMatcher {
	return receiveEnvelope(false, matchers...)
}

func receiveEnvelope(mustHavePrefix bool, matchers ...types.GomegaMatcher) *envelopeReceiver {
	if len(matchers) > 1 {
		panic("ReceiveEnvelope: expected 0 or 1 matchers")
	}
	receiver := &envelopeReceiver{
		mustHavePrefix: mustHavePrefix,
	}
	if len(matchers) == 1 {
		receiver.matcher = matchers[0]
	}
	return receiver
}

func (e *envelopeReceiver) Match(actual interface{}) (success bool, err error) {
	envelope := &events.Envelope{}
	input, ok := actual.(chan []byte)
	if !ok {
		return false, errors.New("Envelope receiver: expected a channel of byte slices")
	}
	var msgBytes []byte
	select {
	case msgBytes = <-input:
		if len(msgBytes) == 0 {
			return false, nil
		}
	default:
		return false, nil
	}

	if err := proto.Unmarshal(msgBytes, envelope); err != nil {
		// Try again, stripping out the length prefix
		if err := proto.Unmarshal(msgBytes[4:], envelope); err != nil {
			return false, nil
		}
		e.hadPrefix = true
	}
	if e.mustHavePrefix && !e.hadPrefix {
		return false, nil
	}
	e.foundEnvelope = envelope
	if e.matcher != nil {
		return e.matcher.Match(envelope)
	}
	return true, nil
}

func (e *envelopeReceiver) FailureMessage(actual interface{}) (message string) {
	if e.foundEnvelope != nil {
		return e.matcher.FailureMessage(e.foundEnvelope)
	}
	if e.mustHavePrefix && !e.hadPrefix {
		return "Expected to receive a size-prefixed []byte"
	}
	return "Expected to receive a []byte which successfully unmarshals to *events.Envelope"
}

func (e *envelopeReceiver) NegatedFailureMessage(actual interface{}) (message string) {
	if e.foundEnvelope != nil {
		return e.matcher.NegatedFailureMessage(e.foundEnvelope)
	}
	if e.mustHavePrefix && e.hadPrefix {
		return "Expected not to receive a size-prefixed []byte"
	}
	return "Expected not to receive a []byte which successfully unmarshals to *events.Envelope"
}
