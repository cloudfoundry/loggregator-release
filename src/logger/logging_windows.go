// +build windows

package logger

import (
	"github.com/cloudfoundry/gosteno"
)

type FakeSink struct{}

func (f FakeSink) AddRecord(r *gosteno.Record)  {}
func (f FakeSink) Flush()                       {}
func (f FakeSink) SetCodec(codec gosteno.Codec) {}
func (f FakeSink) GetCodec() gosteno.Codec      { return nil }

func GetNewSyslogSink(namespace string) gosteno.Sink {
	return FakeSink{}
}
