package batching_test

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/loggregator/plumbing/batching"
)

func ExampleByteBatcher() {
	writer := batching.ByteWriterFunc(func(batch [][]byte) {
		for _, data := range batch {
			fmt.Printf("%s\n", data)
		}
	})
	batcher := batching.NewByteBatcher(100, time.Nanosecond, writer)

	dataSource := make(chan []byte, 5)
	for i := 0; i < 3; i++ {
		dataSource <- []byte(fmt.Sprintf("data %d", i))
	}

	done := time.After(time.Millisecond)
	for {
		// Do a non-blocking read from a data source.
		select {
		case data := <-dataSource:
			// If read succeeds write it out. This will flush if the batch
			// exceeds the batch size.
			batcher.Write(data)
		case <-done:
			return
		default:
			// If read fails make sure to call Flush to ensure data doesn't
			// get stuck in the batch for long periods of time.
			batcher.Flush()
		}
	}

	// Output:
	// data 0
	// data 1
	// data 2
}
