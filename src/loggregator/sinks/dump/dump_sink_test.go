package dump

import (
	"github.com/cloudfoundry/gunk/timeprovider/faketimeprovider"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	messagetesthelpers "github.com/cloudfoundry/loggregatorlib/logmessage/testhelpers"
	"github.com/stretchr/testify/assert"
	"runtime"
	"strconv"
	"testing"
	"time"
)

var fakeTimeProvider = faketimeprovider.New(time.Now())

func TestDumpForOneMessage(t *testing.T) {
	dump := NewDumpSink("myApp", 1, loggertesthelper.Logger(), time.Second, fakeTimeProvider)

	dumpRunnerDone := make(chan struct{})
	inputChan := make(chan *logmessage.Message)

	go func() {
		dump.Run(inputChan)
		close(dumpRunnerDone)
	}()

	logMessage := messagetesthelpers.NewMessage(t, "hi", "appId")
	inputChan <- logMessage

	close(inputChan)
	<-dumpRunnerDone

	data := dump.Dump()
	assert.Equal(t, len(data), 1)
	assert.Equal(t, string(data[0].GetLogMessage().GetMessage()), "hi")
}

func TestDumpForTwoMessages(t *testing.T) {
	dump := NewDumpSink("myApp", 2, loggertesthelper.Logger(), time.Second, fakeTimeProvider)

	dumpRunnerDone := make(chan struct{})
	inputChan := make(chan *logmessage.Message)

	go func() {
		dump.Run(inputChan)
		close(dumpRunnerDone)
	}()

	logMessage := messagetesthelpers.NewMessage(t, "1", "appId")
	inputChan <- logMessage
	logMessage = messagetesthelpers.NewMessage(t, "2", "appId")
	inputChan <- logMessage

	close(inputChan)
	<-dumpRunnerDone

	logMessages := dump.Dump()

	assert.Equal(t, len(logMessages), 2)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "1")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "2")
}

func TestTheDumpSinkNeverFillsUp(t *testing.T) {
	bufferSize := uint32(3)
	dump := NewDumpSink("myApp", bufferSize, loggertesthelper.Logger(), time.Second, fakeTimeProvider)

	dumpRunnerDone := make(chan struct{})
	inputChan := make(chan *logmessage.Message)

	go func() {
		dump.Run(inputChan)
		close(dumpRunnerDone)
	}()

	logMessage := messagetesthelpers.NewMessage(t, "hi", "appId")

	for i := uint32(0); i < bufferSize+1; i++ {
		inputChan <- logMessage
	}

	close(inputChan)
	<-dumpRunnerDone
}

func TestDumpAlwaysReturnsTheNewestMessages(t *testing.T) {
	dump := NewDumpSink("myApp", 2, loggertesthelper.Logger(), time.Second, fakeTimeProvider)

	dumpRunnerDone := make(chan struct{})

	inputChan := make(chan *logmessage.Message)

	go func() {
		dump.Run(inputChan)
		close(dumpRunnerDone)
	}()

	logMessage := messagetesthelpers.NewMessage(t, "1", "appId")
	inputChan <- logMessage
	logMessage = messagetesthelpers.NewMessage(t, "2", "appId")
	inputChan <- logMessage
	logMessage = messagetesthelpers.NewMessage(t, "3", "appId")
	inputChan <- logMessage

	close(inputChan)
	<-dumpRunnerDone

	logMessages := dump.Dump()
	assert.Equal(t, len(logMessages), 2)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "2")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "3")

}

func TestDumpReturnsAllRecentMessagesToMultipleDumpRequests(t *testing.T) {
	dump := NewDumpSink("myApp", 2, loggertesthelper.Logger(), time.Second, fakeTimeProvider)

	dumpRunnerDone := make(chan struct{})
	inputChan := make(chan *logmessage.Message)

	go func() {
		dump.Run(inputChan)
		close(dumpRunnerDone)
	}()

	logMessage := messagetesthelpers.NewMessage(t, "1", "appId")
	inputChan <- logMessage
	logMessage = messagetesthelpers.NewMessage(t, "2", "appId")
	inputChan <- logMessage
	logMessage = messagetesthelpers.NewMessage(t, "3", "appId")
	inputChan <- logMessage

	close(inputChan)
	<-dumpRunnerDone

	logMessages := dump.Dump()
	assert.Equal(t, len(logMessages), 2)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "2")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "3")

	logMessages = dump.Dump()
	assert.Equal(t, len(logMessages), 2)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "2")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "3")
}

func TestDumpReturnsAllRecentMessagesToMultipleDumpRequestsWithMessagesCloningInInTheMeantime(t *testing.T) {
	dump := NewDumpSink("myApp", 2, loggertesthelper.Logger(), time.Second, fakeTimeProvider)

	dumpRunnerDone := make(chan struct{})
	inputChan := make(chan *logmessage.Message)

	go func() {
		dump.Run(inputChan)
		close(dumpRunnerDone)
	}()

	logMessage := messagetesthelpers.NewMessage(t, "1", "appId")
	inputChan <- logMessage
	logMessage = messagetesthelpers.NewMessage(t, "2", "appId")
	inputChan <- logMessage
	logMessage = messagetesthelpers.NewMessage(t, "3", "appId")
	inputChan <- logMessage

	close(inputChan)
	<-dumpRunnerDone

	logMessages := dump.Dump()
	assert.Equal(t, len(logMessages), 2)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "2")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "3")

	dumpRunnerDone = make(chan struct{})
	inputChan = make(chan *logmessage.Message)

	go func() {
		dump.Run(inputChan)
		close(dumpRunnerDone)
	}()

	logMessage = messagetesthelpers.NewMessage(t, "4", "appId")
	inputChan <- logMessage

	logMessages = dump.Dump()
	assert.Equal(t, len(logMessages), 2)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "3")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "4")
}

func TestDumpWithLotsOfMessages(t *testing.T) {
	dump := NewDumpSink("myApp", 2, loggertesthelper.Logger(), time.Second, fakeTimeProvider)

	dumpRunnerDone := make(chan struct{})
	inputChan := make(chan *logmessage.Message)

	go func() {
		dump.Run(inputChan)
		close(dumpRunnerDone)
	}()

	for i := 0; i < 100; i++ {
		logMessage := messagetesthelpers.NewMessage(t, strconv.Itoa(i), "appId")
		inputChan <- logMessage
	}

	close(inputChan)
	<-dumpRunnerDone

	logMessages := dump.Dump()
	assert.Equal(t, len(logMessages), 2)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "98")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "99")

	dumpRunnerDone = make(chan struct{})
	inputChan = make(chan *logmessage.Message)

	go func() {
		dump.Run(inputChan)
		close(dumpRunnerDone)
	}()

	for i := 100; i < 200; i++ {
		logMessage := messagetesthelpers.NewMessage(t, strconv.Itoa(i), "appId")
		inputChan <- logMessage
	}

	close(inputChan)
	<-dumpRunnerDone

	logMessages = dump.Dump()
	assert.Equal(t, len(logMessages), 2)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "198")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "199")

	logMessages = dump.Dump()
	assert.Equal(t, len(logMessages), 2)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "198")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "199")
}

func TestDumpWithLotsOfMessagesAndLargeBuffer(t *testing.T) {
	dump := NewDumpSink("myApp", 200, loggertesthelper.Logger(), time.Second, fakeTimeProvider)

	dumpRunnerDone := make(chan struct{})
	inputChan := make(chan *logmessage.Message)

	go func() {
		dump.Run(inputChan)
		close(dumpRunnerDone)
	}()

	for i := 0; i < 1000; i++ {
		logMessage := messagetesthelpers.NewMessage(t, strconv.Itoa(i), "appId")
		inputChan <- logMessage
	}

	close(inputChan)
	<-dumpRunnerDone

	logMessages := dump.Dump()
	assert.Equal(t, len(logMessages), 200)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "800")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "801")

	dumpRunnerDone = make(chan struct{})
	inputChan = make(chan *logmessage.Message)

	go func() {
		dump.Run(inputChan)
		close(dumpRunnerDone)
	}()

	for i := 1000; i < 2000; i++ {
		logMessage := messagetesthelpers.NewMessage(t, strconv.Itoa(i), "appId")
		inputChan <- logMessage
	}

	close(inputChan)
	<-dumpRunnerDone

	logMessages = dump.Dump()
	assert.Equal(t, len(logMessages), 200)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "1800")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "1801")

	logMessages = dump.Dump()
	assert.Equal(t, len(logMessages), 200)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "1800")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "1801")
}

func TestDumpWithLotsOfMessagesAndLargeBuffer2(t *testing.T) {
	dump := NewDumpSink("myApp", 200, loggertesthelper.Logger(), time.Second, fakeTimeProvider)
	dumpRunnerDone := make(chan struct{})
	inputChan := make(chan *logmessage.Message)

	go func() {
		dump.Run(inputChan)
		close(dumpRunnerDone)
	}()

	for i := 0; i < 100; i++ {
		logMessage := messagetesthelpers.NewMessage(t, strconv.Itoa(i), "appId")
		inputChan <- logMessage
	}

	close(inputChan)
	<-dumpRunnerDone

	logMessages := dump.Dump()
	assert.Equal(t, len(logMessages), 100)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "0")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "1")
	assert.Equal(t, string(logMessages[99].GetLogMessage().GetMessage()), "99")

	dumpRunnerDone = make(chan struct{})
	inputChan = make(chan *logmessage.Message)

	go func() {
		dump.Run(inputChan)
		close(dumpRunnerDone)
	}()

	for i := 100; i < 200; i++ {
		logMessage := messagetesthelpers.NewMessage(t, strconv.Itoa(i), "appId")
		inputChan <- logMessage
	}

	close(inputChan)
	<-dumpRunnerDone
	logMessages = dump.Dump()
	assert.Equal(t, len(logMessages), 200)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "0")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "1")

	dumpRunnerDone = make(chan struct{})
	inputChan = make(chan *logmessage.Message)

	go func() {
		dump.Run(inputChan)
		close(dumpRunnerDone)
	}()

	for i := 200; i < 300; i++ {
		logMessage := messagetesthelpers.NewMessage(t, strconv.Itoa(i), "appId")
		inputChan <- logMessage
	}

	close(inputChan)
	<-dumpRunnerDone
	logMessages = dump.Dump()
	assert.Equal(t, len(logMessages), 200)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "100")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "101")
}

func TestDumpWithLotsOfDumps(t *testing.T) {
	runtime.GOMAXPROCS(runtime.NumCPU())
	dump := NewDumpSink("myApp", 5, loggertesthelper.Logger(), time.Second, fakeTimeProvider)
	dumpRunnerDone := make(chan struct{})
	inputChan := make(chan *logmessage.Message)

	go func() {
		dump.Run(inputChan)
		close(dumpRunnerDone)
	}()

	for i := 0; i < 10; i++ {
		logMessage := messagetesthelpers.NewMessage(t, strconv.Itoa(i), "appId")
		inputChan <- logMessage
	}

	close(inputChan)
	<-dumpRunnerDone

	for i := 0; i < 200; i++ {
		go func() {
			logMessages := dump.Dump()

			assert.Equal(t, len(logMessages), 5)
			assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "5")
			assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "6")
		}()
	}
}

func TestDumpSinkClosesItselfAfterPeriodOfInactivity(t *testing.T) {
	dump := NewDumpSink("myApp", 5, loggertesthelper.Logger(), 2*time.Millisecond, fakeTimeProvider)
	dumpRunnerDone := make(chan struct{})
	inputChan := make(chan *logmessage.Message)

	go func() {
		dump.Run(inputChan)
		close(dumpRunnerDone)
	}()

	select {
	case <-dumpRunnerDone:
		// OK
	case <-time.After(5 * time.Millisecond):
		assert.Fail(t, "Should have timeouted the dump")
	}
}

func xTestDumpSinkClosingTimeIsResetWhenAMessageArrives(t *testing.T) {
	dump := NewDumpSink("myApp", 5, loggertesthelper.Logger(), 10*time.Millisecond, fakeTimeProvider)

	dumpRunnerDone := make(chan struct{})
	inputChan := make(chan *logmessage.Message)

	go func() {
		dump.Run(inputChan)
		close(dumpRunnerDone)
	}()

	fakeTimeProvider.IncrementBySeconds(uint64(5))
	logMessage := messagetesthelpers.NewMessage(t, "0", "appId")
	inputChan <- logMessage
	fakeTimeProvider.IncrementBySeconds(uint64(8))

	fakeTimeProvider.IncrementBySeconds(uint64(3))

	select {
	case sink := <-dumpRunnerDone:
		assert.Equal(t, sink, dump)
	case <-time.After(5 * time.Millisecond):
		assert.Fail(t, "Should have closed")
	}
}
