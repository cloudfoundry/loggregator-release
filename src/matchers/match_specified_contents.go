package matchers

import (
	"fmt"
	"reflect"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
)

func MatchSpecifiedContents(expected *events.Envelope) types.GomegaMatcher {
	return &specifiedContentsMatcher{
		expected: expected,
	}
}

type specifiedContentsMatcher struct {
	expected      *events.Envelope
	failureReason string
}

func (matcher *specifiedContentsMatcher) Match(actual interface{}) (bool, error) {
	if _, ok := actual.(*events.Envelope); !ok {
		return false, fmt.Errorf("MatchSpecifiedContents matcher expects actual to be an *events.Envelope")
	}
	return matcher.matchSpecifiedContents(actual, matcher.expected)
}

func (matcher *specifiedContentsMatcher) FailureMessage(actual interface{}) string {
	return fmt.Sprintf("Expected\n\t%#s\nto equal non-nil contents of\n\t%#s\nReason for failure: %s\n", format.Object(actual, 1), format.Object(matcher.expected, 1), matcher.failureReason)
}

func (matcher *specifiedContentsMatcher) NegatedFailureMessage(actual interface{}) string {
	return fmt.Sprintf("Expected\n\t%#s not to equal non-nil contents of\n\t%#s\n", format.Object(actual, 1), format.Object(matcher.expected, 1))
}

func (matcher *specifiedContentsMatcher) matchSpecifiedContents(actualInter, expectedInter interface{}) (bool, error) {
	actual := reflect.ValueOf(actualInter)
	expected := reflect.ValueOf(expectedInter)
	switch expected.Kind() {
	case reflect.Ptr:
		if expected.IsNil() {
			return true, nil
		}
		return matcher.matchSpecifiedContents(actual.Elem().Interface(), expected.Elem().Interface())
	case reflect.Struct:
		for i := 0; i < expected.NumField(); i++ {
			success, err := matcher.matchSpecifiedContents(actual.Field(i).Interface(), expected.Field(i).Interface())
			if !success || err != nil {
				return success, err
			}
		}
		return true, nil
	default:
		if !reflect.DeepEqual(actualInter, expectedInter) {
			matcher.failureReason = fmt.Sprintf("%#v != %#v", actualInter, expectedInter)
			return false, nil
		}
		return true, nil
	}
}
