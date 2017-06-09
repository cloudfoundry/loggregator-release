package listeners

import "github.com/cloudfoundry/dropsonde/metricbatcher"

//go:generate hel --type Batcher --output mock_batcher_test.go

type Batcher interface {
	BatchCounter(name string) metricbatcher.BatchCounterChainer
	BatchIncrementCounter(name string)
	BatchAddCounter(name string, delta uint64)
}
