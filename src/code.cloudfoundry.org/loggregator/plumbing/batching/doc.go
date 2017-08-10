// Package batching provides mechanisms for batching writes of various types.
// A batcher's methods should be invoked from a single goroutine. It is the
// responsibility of the caller to invoke Flush on the batcher frequently to
// flush the current batch out to the writer.
package batching
