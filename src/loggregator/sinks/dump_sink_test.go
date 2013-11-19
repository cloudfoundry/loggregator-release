package sinks

import (
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	messagetesthelpers "github.com/cloudfoundry/loggregatorlib/logmessage/testhelpers"
	"github.com/stretchr/testify/assert"
	"runtime"
	"strconv"
	"testing"
)

func dumpAllMessages(dump *DumpSink) []*logmessage.Message {
	return dump.Dump()
}

func TestDumpForOneMessage(t *testing.T) {
	dump := NewDumpSink("myApp", 1, loggertesthelper.Logger())
	dump.Run()

	logMessage := messagetesthelpers.NewMessage(t, "hi", "appId")
	dump.Channel() <- logMessage

	logMessages := dumpAllMessages(dump)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "hi")
}

func TestDumpForTwoMessages(t *testing.T) {
	dump := NewDumpSink("myApp", 2, loggertesthelper.Logger())
	dump.Run()

	logMessage := messagetesthelpers.NewMessage(t, "1", "appId")
	dump.Channel() <- logMessage
	logMessage = messagetesthelpers.NewMessage(t, "2", "appId")
	dump.Channel() <- logMessage

	runtime.Gosched()

	logMessages := dumpAllMessages(dump)
	assert.Equal(t, len(logMessages), 2)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "1")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "2")
}

func TestTheDumpSinkNeverFillsUp(t *testing.T) {
	bufferSize := 3
	dump := NewDumpSink("myApp", bufferSize, loggertesthelper.Logger())
	dump.Run()

	logMessage := messagetesthelpers.NewMessage(t, "hi", "appId")

	for i := 0; i < bufferSize+1; i++ {
		dump.Channel() <- logMessage
	}
}

func TestDumpAlwaysReturnsTheNewestMessages(t *testing.T) {
	dump := NewDumpSink("myApp", 2, loggertesthelper.Logger())
	dump.Run()

	logMessage := messagetesthelpers.NewMessage(t, "1", "appId")
	dump.Channel() <- logMessage
	logMessage = messagetesthelpers.NewMessage(t, "2", "appId")
	dump.Channel() <- logMessage
	logMessage = messagetesthelpers.NewMessage(t, "3", "appId")
	dump.Channel() <- logMessage

	runtime.Gosched()

	logMessages := dumpAllMessages(dump)
	assert.Equal(t, len(logMessages), 2)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "2")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "3")
}

func TestDumpReturnsAllRecentMessagesToMultipleDumpRequests(t *testing.T) {
	dump := NewDumpSink("myApp", 2, loggertesthelper.Logger())
	dump.Run()

	logMessage := messagetesthelpers.NewMessage(t, "1", "appId")
	dump.Channel() <- logMessage
	logMessage = messagetesthelpers.NewMessage(t, "2", "appId")
	dump.Channel() <- logMessage
	logMessage = messagetesthelpers.NewMessage(t, "3", "appId")
	dump.Channel() <- logMessage

	runtime.Gosched()

	logMessages := dumpAllMessages(dump)
	assert.Equal(t, len(logMessages), 2)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "2")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "3")

	logMessages = dumpAllMessages(dump)
	assert.Equal(t, len(logMessages), 2)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "2")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "3")
}

func TestDumpReturnsAllRecentMessagesToMultipleDumpRequestsWithMessagesCloningInInTheMeantime(t *testing.T) {

	dump := NewDumpSink("myApp", 2, loggertesthelper.Logger())
	dump.Run()

	logMessage := messagetesthelpers.NewMessage(t, "1", "appId")
	dump.Channel() <- logMessage
	logMessage = messagetesthelpers.NewMessage(t, "2", "appId")
	dump.Channel() <- logMessage
	logMessage = messagetesthelpers.NewMessage(t, "3", "appId")
	dump.Channel() <- logMessage

	runtime.Gosched()

	logMessages := dumpAllMessages(dump)
	assert.Equal(t, len(logMessages), 2)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "2")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "3")

	logMessage = messagetesthelpers.NewMessage(t, "4", "appId")
	dump.Channel() <- logMessage

	runtime.Gosched()

	logMessages = dumpAllMessages(dump)
	assert.Equal(t, len(logMessages), 2)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "3")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "4")
}

func TestDumpWithLotsOfMessages(t *testing.T) {
	dump := NewDumpSink("myApp", 2, loggertesthelper.Logger())
	dump.Run()

	for i := 0; i < 100; i++ {
		logMessage := messagetesthelpers.NewMessage(t, strconv.Itoa(i), "appId")
		dump.Channel() <- logMessage
	}

	runtime.Gosched()

	logMessages := dumpAllMessages(dump)
	assert.Equal(t, len(logMessages), 2)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "98")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "99")

	for i := 100; i < 200; i++ {
		logMessage := messagetesthelpers.NewMessage(t, strconv.Itoa(i), "appId")
		dump.Channel() <- logMessage
	}

	runtime.Gosched()

	logMessages = dumpAllMessages(dump)
	assert.Equal(t, len(logMessages), 2)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "198")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "199")

	logMessages = dumpAllMessages(dump)
	assert.Equal(t, len(logMessages), 2)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "198")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "199")
}

func TestDumpWithLotsOfMessagesAndLargeBuffer(t *testing.T) {
	dump := NewDumpSink("myApp", 200, loggertesthelper.Logger())
	dump.Run()

	for i := 0; i < 1000; i++ {
		logMessage := messagetesthelpers.NewMessage(t, strconv.Itoa(i), "appId")
		dump.Channel() <- logMessage
	}

	runtime.Gosched()

	logMessages := dumpAllMessages(dump)
	assert.Equal(t, len(logMessages), 200)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "800")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "801")

	for i := 1000; i < 2000; i++ {
		logMessage := messagetesthelpers.NewMessage(t, strconv.Itoa(i), "appId")
		dump.Channel() <- logMessage
	}

	runtime.Gosched()

	logMessages = dumpAllMessages(dump)
	assert.Equal(t, len(logMessages), 200)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "1800")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "1801")

	logMessages = dumpAllMessages(dump)
	assert.Equal(t, len(logMessages), 200)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "1800")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "1801")
}

func TestDumpWithLotsOfMessagesAndLargeBuffer2(t *testing.T) {
	dump := NewDumpSink("myApp", 200, loggertesthelper.Logger())
	dump.Run()

	for i := 0; i < 100; i++ {
		logMessage := messagetesthelpers.NewMessage(t, strconv.Itoa(i), "appId")
		dump.Channel() <- logMessage
	}

	runtime.Gosched()

	logMessages := dumpAllMessages(dump)
	assert.Equal(t, len(logMessages), 100)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "0")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "1")

	for i := 100; i < 200; i++ {
		logMessage := messagetesthelpers.NewMessage(t, strconv.Itoa(i), "appId")
		dump.Channel() <- logMessage
	}

	runtime.Gosched()

	logMessages = dumpAllMessages(dump)
	assert.Equal(t, len(logMessages), 200)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "0")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "1")

	for i := 200; i < 300; i++ {
		logMessage := messagetesthelpers.NewMessage(t, strconv.Itoa(i), "appId")
		dump.Channel() <- logMessage
	}

	runtime.Gosched()

	logMessages = dumpAllMessages(dump)
	assert.Equal(t, len(logMessages), 200)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "100")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "101")
}

func TestDumpWithLotsOfDumps(t *testing.T) {
	runtime.GOMAXPROCS(runtime.NumCPU())
	dump := NewDumpSink("myApp", 5, loggertesthelper.Logger())
	dump.Run()

	for i := 0; i < 10; i++ {
		logMessage := messagetesthelpers.NewMessage(t, strconv.Itoa(i), "appId")
		dump.Channel() <- logMessage
	}

	runtime.Gosched()

	for i := 0; i < 200; i++ {
		go func() {
			logMessages := dumpAllMessages(dump)

			assert.Equal(t, len(logMessages), 5)
			assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "5")
			assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "6")
		}()
	}
}
