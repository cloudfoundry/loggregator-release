package sinks

import (
	"github.com/stretchr/testify/assert"
	"runtime"
	"testing"
	"time"
)

func TestThatOnlyOneRequestCloseOccurs(t *testing.T) {
	closeChan := make(chan Sink)

	sink := testSink{"1"}
	go sink.Run(closeChan)
	runtime.Gosched()

	closeSink := <-closeChan
	assert.Equal(t, &sink, closeSink)

	select {
	case <-closeChan:
		t.Error("Should not have received value on closeChan")
	case <-time.After(50 * time.Millisecond):
	}

}
