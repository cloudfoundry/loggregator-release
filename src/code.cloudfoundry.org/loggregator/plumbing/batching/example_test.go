package batching_test

import (
	"time"

	"code.cloudfoundry.org/loggregator/plumbing/batching"
)

func ExampleBatcher() {
	dataSource := make(chan []byte)
	writer := batching.WriterFunc(func(batch []interface{}) {
		// Write the batch out to the network.
	})
	batcher := batching.NewBatcher(100, time.Second, writer)

	for {
		// Do a non-blocking read from a data source.
		select {
		case data := <-dataSource:
			// If read succeeds write it out. This will flush if the batch
			// exceeds the batch size.
			batcher.Write(data)
		default:
			// If read fails make sure to call Flush to ensure data doesn't
			// get stuck in the batch for long periods of time.
			batcher.Flush()
		}
	}
}
