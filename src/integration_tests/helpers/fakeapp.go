package helpers

import (
	"tools/benchmark/experiment"
	"tools/benchmark/messagegenerator"
	"tools/benchmark/messagewriter"
	"tools/benchmark/metricsreporter"
	"tools/benchmark/writestrategies"
)

type FakeApp struct {
	appID         string
	logRate       int
	writeStrategy experiment.WriteStrategy
	logCounter    *metricsreporter.Counter
}

func NewFakeApp(appID string, logRate int) *FakeApp {

	generator := messagegenerator.NewLogMessageGenerator(appID)
	counter := metricsreporter.NewCounter("sentLogs")
	// 49625 is the DropsondeIncomingMessagesPort for the metron that will be spun up.
	writer := messagewriter.NewMessageWriter("localhost", 49625, "", counter)

	writeStrategy := writestrategies.NewConstantWriteStrategy(generator, writer, logRate)

	return &FakeApp{
		appID:         appID,
		logRate:       logRate,
		writeStrategy: writeStrategy,
		logCounter:    counter,
	}
}

func (f *FakeApp) Start(start chan struct{}) {
	<-start
	f.writeStrategy.StartWriter()
}

func (f *FakeApp) SentLogs() uint64 {
	return f.logCounter.GetValue()
}

func (f *FakeApp) AppID() string {
	return f.appID
}

func (f *FakeApp) Stop() {
	f.writeStrategy.Stop()
}
