package matchers

import (
	"fmt"
	"reflect"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
)

func MatchSpecifiedContents(expected interface{}) types.GomegaMatcher {
	return &specifiedContentsMatcher{
		expected: expected,
	}
}

type specifiedContentsMatcher struct {
	expected      interface{}
	failureReason string
}

func (matcher *specifiedContentsMatcher) Match(actual interface{}) (success bool, err error) {
	actualEnvelope, ok := actual.(*events.Envelope)
	if !ok {
		return false, fmt.Errorf("MatchEnvelopeUpToTimestamp matcher expects actual to be an *events.Envelope")
	}

	expectedEnvelope, ok := matcher.expected.(*events.Envelope)
	if !ok {
		return false, fmt.Errorf("MatchEnvelopeUpToTimestamp matcher expects expected to be an *events.Envelope")
	}

	if expectedEnvelope.Origin != nil && actualEnvelope.GetOrigin() != expectedEnvelope.GetOrigin() {
		matcher.failureReason = "Origin did not match"
		return false, nil
	}
	if expectedEnvelope.EventType != nil && actualEnvelope.GetEventType() != expectedEnvelope.GetEventType() {
		matcher.failureReason = "EventType did not match"
		return false, nil
	}
	if expectedEnvelope.Deployment != nil && actualEnvelope.GetDeployment() != expectedEnvelope.GetDeployment() {
		matcher.failureReason = "Deployment did not match"
		return false, nil
	}
	if expectedEnvelope.Job != nil && actualEnvelope.GetJob() != expectedEnvelope.GetJob() {
		matcher.failureReason = "Job did not match"
		return false, nil
	}
	if expectedEnvelope.Index != nil && actualEnvelope.GetIndex() != expectedEnvelope.GetIndex() {
		matcher.failureReason = "Index did not match"
		return false, nil
	}
	if expectedEnvelope.Ip != nil && actualEnvelope.GetIp() != expectedEnvelope.GetIp() {
		matcher.failureReason = "Ip did not match"
		return false, nil
	}
	if expectedEnvelope.HttpStart != nil && reflect.DeepEqual(actualEnvelope.GetHttpStart(), expectedEnvelope.GetHttpStart()) {
		matcher.failureReason = "HttpStart did not match"
		return false, nil
	}
	if expectedEnvelope.HttpStop != nil && reflect.DeepEqual(actualEnvelope.GetHttpStop(), expectedEnvelope.GetHttpStop()) {
		matcher.failureReason = "HttpStop did not match"
		return false, nil
	}
	if expectedEnvelope.HttpStartStop != nil && reflect.DeepEqual(actualEnvelope.GetHttpStartStop(), expectedEnvelope.GetHttpStartStop()) {
		matcher.failureReason = "HttpStartStop did not match"
		return false, nil
	}
	if expectedEnvelope.LogMessage != nil && reflect.DeepEqual(actualEnvelope.GetLogMessage(), expectedEnvelope.GetLogMessage()) {
		matcher.failureReason = "LogMessage did not match"
		return false, nil
	}
	if expectedEnvelope.ValueMetric != nil && reflect.DeepEqual(actualEnvelope.GetValueMetric(), expectedEnvelope.GetValueMetric()) {
		matcher.failureReason = "ValueMetric did not match"
		return false, nil
	}
	if expectedEnvelope.CounterEvent != nil && !reflect.DeepEqual(actualEnvelope.GetCounterEvent(), expectedEnvelope.GetCounterEvent()) {
		matcher.failureReason = "CounterEvent did not match"
		return false, nil
	}
	if expectedEnvelope.Error != nil && reflect.DeepEqual(actualEnvelope.GetError(), expectedEnvelope.GetError()) {
		matcher.failureReason = "Error did not match"
		return false, nil
	}
	if expectedEnvelope.ContainerMetric != nil && reflect.DeepEqual(actualEnvelope.GetContainerMetric(), expectedEnvelope.GetContainerMetric()) {
		matcher.failureReason = "ContainerMetric did not match"
		return false, nil
	}

	return true, nil
}

func (matcher *specifiedContentsMatcher) FailureMessage(actual interface{}) string {
	return fmt.Sprintf("Expected\n\t%#s\nto equal non-nil contents of\n\t%#s\nReason for failure: %s\n", format.Object(actual, 1), format.Object(matcher.expected, 1), matcher.failureReason)
}

func (matcher *specifiedContentsMatcher) NegatedFailureMessage(actual interface{}) string {
	return fmt.Sprintf("Expected\n\t%#s not to equal non-nil contents of\n\t%#s\n", format.Object(actual, 1), format.Object(matcher.expected, 1))
}
