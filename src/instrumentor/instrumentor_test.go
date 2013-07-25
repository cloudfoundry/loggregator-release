package instrumentor

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestInstrumentation(t *testing.T) {
	logger := NewFakeLogger()
	instrumentor := NewInstrumentor(10*time.Millisecond, gosteno.LOG_ALL, logger)

	testSimpleDumper := SimpleDumper{"PropertyX", "some data to dump"}

	instrumentor.Instrument(testSimpleDumper)

	select {
	case msg := <-logger.LogMessages:
		assert.Equal(t, msg, "PropertyX: some data to dump")
	case <-time.After(100 * time.Millisecond):
		t.Error("Instrumentor should have instrumented in the meantime!")
	}
}

func TestIntervalOfInstrumentation(t *testing.T) {
	logger := NewFakeLogger()
	instrumentor := NewInstrumentor(10*time.Millisecond, gosteno.LOG_ALL, logger)

	testSimpleDumper := SimpleDumper{"PropertyX", "some data to dump"}

	instrumentor.Instrument(testSimpleDumper)

	for i := 0; i < 3; i++ {
		time.Sleep(12 * time.Millisecond)
		select {
		case msg := <-logger.LogMessages:
			assert.Equal(t, msg, "PropertyX: some data to dump")
		default:
			t.Error("Instrumentor should have instrumented in the meantime!")
		}
	}
}

func TestNonInstrumentationDueToDebugLevel(t *testing.T) {
	logger := NewFakeLogger()
	instrumentor := NewInstrumentor(1*time.Millisecond, gosteno.LOG_OFF, logger)

	testSimpleDumper := SimpleDumper{"PropertyX", "some data to dump"}

	instrumentor.Instrument(testSimpleDumper)

	select {
	case msg := <-logger.LogMessages:
		t.Error("Instrumentor should not have instrumented due to log level! Got: %v", msg)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestStopInstrumentation(t *testing.T) {
	logger := NewFakeLogger()
	instrumentor := NewInstrumentor(20*time.Millisecond, gosteno.LOG_ALL, logger)

	testSimpleDumper := SimpleDumper{"PropertyX", "some data to dump"}

	stopChan := instrumentor.Instrument(testSimpleDumper)

	select {
	case msg := <-logger.LogMessages:
		assert.Equal(t, msg, "PropertyX: some data to dump")
	case <-time.After(25 * time.Millisecond):
		t.Error("Instrumentor should have instrumented in the meantime!")
	}

	instrumentor.StopInstrumentation(stopChan)

	select {
	case msg := <-logger.LogMessages:
		t.Error("Instrumentor should not have instrumented due to shutdown! Got: %v", msg)
	case <-time.After(200 * time.Millisecond):
	}
}

type FakeLogger struct {
	LogMessages chan string
	LogLevel    gosteno.LogLevel
}

func (l *FakeLogger) Level() gosteno.LogLevel {
	return l.LogLevel
}

func (l *FakeLogger) Log(x gosteno.LogLevel, m string, d map[string]interface{}) {
	if l.LogLevel.Priority >= x.Priority {
		l.LogMessages <- m
	}
}

func NewFakeLogger() *FakeLogger {
	fakeLogger := &FakeLogger{LogLevel: gosteno.LOG_ALL}
	fakeLogger.LogMessages = make(chan string)
	return fakeLogger
}

type SimpleDumper struct {
	DumpabableProperty string
	DumpabableValue    string
}

func (s SimpleDumper) DumpData() []PropVal {
	return []PropVal{PropVal{s.DumpabableProperty, s.DumpabableValue}}
}
