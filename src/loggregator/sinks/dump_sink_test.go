package sinks

import (
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	messagetesthelpers "github.com/cloudfoundry/loggregatorlib/logmessage/testhelpers"
	"github.com/stretchr/testify/assert"
	"runtime"
	"strconv"
	"testing"
	"time"
	"sync"
)

func TestDumpForOneMessage(t *testing.T) {
	dump := NewDumpSink("myApp", 1, loggertesthelper.Logger(), make(chan Sink, 1), time.Second)

	go dump.Run()

	logMessage := messagetesthelpers.NewMessage(t, "hi", "appId")
	dump.Channel() <- logMessage

	data := dump.Dump()
	assert.Equal(t, len(data), 1)
	assert.Equal(t, string(data[0].GetLogMessage().GetMessage()), "hi")
}

func TestDumpForTwoMessages(t *testing.T) {
	dump := NewDumpSink("myApp", 2, loggertesthelper.Logger(), make(chan Sink, 1), time.Second)

	go dump.Run()

	logMessage := messagetesthelpers.NewMessage(t, "1", "appId")
	dump.Channel() <- logMessage
	logMessage = messagetesthelpers.NewMessage(t, "2", "appId")
	dump.Channel() <- logMessage

	runtime.Gosched()

	logMessages := dump.Dump()
	assert.Equal(t, len(logMessages), 2)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "1")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "2")
}

func TestTheDumpSinkNeverFillsUp(t *testing.T) {
	bufferSize := 3
	dump := NewDumpSink("myApp", bufferSize, loggertesthelper.Logger(), make(chan Sink, 1), time.Second)

	go dump.Run()

	logMessage := messagetesthelpers.NewMessage(t, "hi", "appId")

	for i := 0; i < bufferSize+1; i++ {
		dump.Channel() <- logMessage
	}
}

func TestDumpAlwaysReturnsTheNewestMessages(t *testing.T) {
	dump := NewDumpSink("myApp", 2, loggertesthelper.Logger(), make(chan Sink, 1), time.Second)

	go dump.Run()

	logMessage := messagetesthelpers.NewMessage(t, "1", "appId")
	dump.Channel() <- logMessage
	logMessage = messagetesthelpers.NewMessage(t, "2", "appId")
	dump.Channel() <- logMessage
	logMessage = messagetesthelpers.NewMessage(t, "3", "appId")
	dump.Channel() <- logMessage

	runtime.Gosched()

	logMessages := dump.Dump()
	assert.Equal(t, len(logMessages), 2)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "2")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "3")
}

func TestDumpReturnsAllRecentMessagesToMultipleDumpRequests(t *testing.T) {
	dump := NewDumpSink("myApp", 2, loggertesthelper.Logger(), make(chan Sink, 1), time.Second)

	go dump.Run()

	logMessage := messagetesthelpers.NewMessage(t, "1", "appId")
	dump.Channel() <- logMessage
	logMessage = messagetesthelpers.NewMessage(t, "2", "appId")
	dump.Channel() <- logMessage
	logMessage = messagetesthelpers.NewMessage(t, "3", "appId")
	dump.Channel() <- logMessage

	runtime.Gosched()

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
	dump := NewDumpSink("myApp", 2, loggertesthelper.Logger(), make(chan Sink, 1), time.Second)

	go dump.Run()

	logMessage := messagetesthelpers.NewMessage(t, "1", "appId")
	dump.Channel() <- logMessage
	logMessage = messagetesthelpers.NewMessage(t, "2", "appId")
	dump.Channel() <- logMessage
	logMessage = messagetesthelpers.NewMessage(t, "3", "appId")
	dump.Channel() <- logMessage

	runtime.Gosched()

	logMessages := dump.Dump()
	assert.Equal(t, len(logMessages), 2)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "2")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "3")

	logMessage = messagetesthelpers.NewMessage(t, "4", "appId")
	dump.Channel() <- logMessage

	runtime.Gosched()

	logMessages = dump.Dump()
	assert.Equal(t, len(logMessages), 2)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "3")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "4")
}

func TestDumpWithLotsOfMessages(t *testing.T) {
	dump := NewDumpSink("myApp", 2, loggertesthelper.Logger(), make(chan Sink, 1), time.Second)

	go dump.Run()

	for i := 0; i < 100; i++ {
		logMessage := messagetesthelpers.NewMessage(t, strconv.Itoa(i), "appId")
		dump.Channel() <- logMessage
	}

	runtime.Gosched()

	logMessages := dump.Dump()
	assert.Equal(t, len(logMessages), 2)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "98")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "99")

	for i := 100; i < 200; i++ {
		logMessage := messagetesthelpers.NewMessage(t, strconv.Itoa(i), "appId")
		dump.Channel() <- logMessage
	}

	runtime.Gosched()

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
	dump := NewDumpSink("myApp", 200, loggertesthelper.Logger(), make(chan Sink, 1), time.Second)

	go dump.Run()

	for i := 0; i < 1000; i++ {
		logMessage := messagetesthelpers.NewMessage(t, strconv.Itoa(i), "appId")
		dump.Channel() <- logMessage
	}

	runtime.Gosched()

	logMessages := dump.Dump()
	assert.Equal(t, len(logMessages), 200)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "800")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "801")

	for i := 1000; i < 2000; i++ {
		logMessage := messagetesthelpers.NewMessage(t, strconv.Itoa(i), "appId")
		dump.Channel() <- logMessage
	}

	runtime.Gosched()

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
	dump := NewDumpSink("myApp", 200, loggertesthelper.Logger(), make(chan Sink, 1), time.Second)
	go dump.Run()

	for i := 0; i < 100; i++ {
		logMessage := messagetesthelpers.NewMessage(t, strconv.Itoa(i), "appId")
		dump.Channel() <- logMessage
	}

	time.Sleep(10 * time.Millisecond)

	logMessages := dump.Dump()
	assert.Equal(t, len(logMessages), 100)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "0")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "1")

	for i := 100; i < 200; i++ {
		logMessage := messagetesthelpers.NewMessage(t, strconv.Itoa(i), "appId")
		dump.Channel() <- logMessage
	}

	time.Sleep(10 * time.Millisecond)

	logMessages = dump.Dump()
	assert.Equal(t, len(logMessages), 200)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "0")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "1")

	for i := 200; i < 300; i++ {
		logMessage := messagetesthelpers.NewMessage(t, strconv.Itoa(i), "appId")
		dump.Channel() <- logMessage
	}

	time.Sleep(10 * time.Millisecond)

	logMessages = dump.Dump()
	assert.Equal(t, len(logMessages), 200)
	assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "100")
	assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "101")
}

func TestDumpWithLotsOfDumps(t *testing.T) {
	runtime.GOMAXPROCS(runtime.NumCPU())
	dump := NewDumpSink("myApp", 5, loggertesthelper.Logger(), make(chan Sink, 1), time.Second)
	go dump.Run()

	for i := 0; i < 10; i++ {
		logMessage := messagetesthelpers.NewMessage(t, strconv.Itoa(i), "appId")
		dump.Channel() <- logMessage
	}

	time.Sleep(10 * time.Millisecond)

	for i := 0; i < 200; i++ {
		go func() {
			logMessages := dump.Dump()

			assert.Equal(t, len(logMessages), 5)
			assert.Equal(t, string(logMessages[0].GetLogMessage().GetMessage()), "5")
			assert.Equal(t, string(logMessages[1].GetLogMessage().GetMessage()), "6")
		}()
	}
}

func TestClosingInputChanAlsoClosesPassThruChan(t *testing.T) {
	dump := NewDumpSink("myApp", 5, loggertesthelper.Logger(), make(chan Sink, 1), time.Second)
	go dump.Run()

	close(dump.Channel())

	select {
	case _, open := <-dump.passThruChan:
		assert.False(t, open)
	case <-time.After(time.Second):
		assert.Fail(t, "Expected the pass thru channel to have been closed")
	}
}

func TestDumpSinkClosesItselfAfterPeriodOfInactivity(t *testing.T) {
	timeoutChan := make(chan Sink, 1)
	dump := NewDumpSink("myApp", 5, loggertesthelper.Logger(), timeoutChan, 10*time.Millisecond)
	wait := sync.WaitGroup{}
	wait.Add(1)
	go func() {
		dump.Run()
		wait.Done()
	}()

	time.Sleep(9 * time.Millisecond)
	assert.Equal(t, len(timeoutChan), 0)

	wait.Wait()
	assert.Equal(t, len(timeoutChan), 1)

	select {
	case sink := <-timeoutChan:
		assert.Equal(t, sink, dump)
	case <-time.After(5 * time.Millisecond):
		assert.Fail(t, "Should have gotten data on timeoutChan")
	}
}

func TestDumpSinkClosingTimeIsResetWhenAMessageArrives(t *testing.T) {
	timeoutChan := make(chan Sink, 1)
	dump := NewDumpSink("myApp", 5, loggertesthelper.Logger(), timeoutChan, 10*time.Millisecond)
	go dump.Run()

	time.Sleep(5 * time.Millisecond)

	logMessage := messagetesthelpers.NewMessage(t, "0", "appId")
	dump.Channel() <- logMessage

	time.Sleep(8 * time.Millisecond)
	assert.Equal(t, len(timeoutChan), 0)

	time.Sleep(3 * time.Millisecond)
	assert.Equal(t, len(timeoutChan), 1)
	select {
	case sink := <-timeoutChan:
		assert.Equal(t, sink, dump)
	case <-time.After(5 * time.Millisecond):
		assert.Fail(t, "Should have closed")
	}
}
