package helpers

import (
	"tools/benchmark/experiment"
	"tools/benchmark/messagegenerator"
	"tools/benchmark/messagewriter"
	"tools/benchmark/metricsreporter"
	"tools/benchmark/writestrategies"
)

type FakeApp struct {
	appID          string
	logRate        int
	writeStrategy  experiment.WriteStrategy
	warmupStrategy experiment.WriteStrategy
	logCounter     *metricsreporter.Counter
}

func NewFakeApp(appID string, warmupRate int, logRate int) *FakeApp {
	generator := messagegenerator.NewLogMessageGenerator(appID)
	counter := metricsreporter.NewCounter("sentLogs")
	// 49625 is the IncomingUDPPort for the metron that will be spun up.
	writer := messagewriter.NewMessageWriter("localhost", 49625, "", counter)

	writeStrategy := writestrategies.NewConstantWriteStrategy(generator, writer, logRate)
	warmupStrategy := writestrategies.NewConstantWriteStrategy(generator, writer, warmupRate)

	return &FakeApp{
		appID:          appID,
		logRate:        logRate,
		writeStrategy:  writeStrategy,
		warmupStrategy: warmupStrategy,
		logCounter:     counter,
	}
}

func (f *FakeApp) Warmup() {
	f.warmupStrategy.StartWriter()
}

func (f *FakeApp) Start() {
	f.warmupStrategy.Stop()
	f.logCounter.Reset()

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
