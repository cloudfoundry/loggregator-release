package loggregator

import (
	"github.com/cloudfoundry/gosteno"

//	"os"
)

func logger() *gosteno.Logger {

	//	loggingConfig := &gosteno.Config{
	//		Sinks:     make([]gosteno.Sink, 1),
	//		Level:     gosteno.LOG_DEBUG,
	//		Codec:     gosteno.NewJsonCodec(),
	//		EnableLOC: true}
	//
	//	loggingConfig.Sinks[0] = gosteno.NewIOSink(os.Stdout)
	//	gosteno.Init(loggingConfig)
	//	return gosteno.NewLogger("TestLoggregator")

	return gosteno.NewLogger("TestLoggregator")
}
