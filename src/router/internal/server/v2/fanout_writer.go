package v2

import (
	"context"
	"sync"

	"code.cloudfoundry.org/go-loggregator/v10/rpc/loggregator_v2"
)

// FanoutWriter fans out Write calls to N concurrent worker goroutines all
// reading from a single shared channel. Its Write method satisfies the Writer
// function signature, allowing it to be used with NewRepeater via f.Write.
type FanoutWriter struct {
	w       Writer
	ch      chan *loggregator_v2.Envelope
	workers int
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
}

// FanoutOption configures a FanoutWriter.
type FanoutOption func(*FanoutWriter)

// WithBufferSize overrides the channel buffer size. The default is two slot
// per worker. Use a larger value to decouple producer speed from worker
// throughput.
func WithBufferSize(n int) FanoutOption {
	return func(f *FanoutWriter) {
		f.ch = make(chan *loggregator_v2.Envelope, n)
	}
}

// NewFanoutWriter creates a FanoutWriter with concurrent publisher
// goroutines (workers) sharing a single channel. Call Start() before writing. The
// channel buffer defaults to two slot per worker; use WithBufferSize to
// override.
func NewFanoutWriter(w Writer, workers int, opts ...FanoutOption) *FanoutWriter {
	ctx, cancel := context.WithCancel(context.Background())
	f := &FanoutWriter{
		w:       w,
		ch:      make(chan *loggregator_v2.Envelope, workers*2),
		workers: workers,
		ctx:     ctx,
		cancel:  cancel,
	}
	for _, o := range opts {
		o(f)
	}
	return f
}

// Start launches the worker goroutines. Must be called before Write.
func (f *FanoutWriter) Start() {
	for range f.workers {
		f.wg.Add(1)
		go f.writeWorker()
	}
}

// Stop cancels the context (unblocking any Write blocked on ctx.Done), closes
// the channel, and blocks until all workers have returned. Workers drain any
// envelopes remaining in the buffer before exiting.
func (f *FanoutWriter) Stop() {
	f.cancel()
	close(f.ch)
	f.wg.Wait()
}

// Write sends e to the shared channel, blocking if the channel is full.
// Drops e silently if the context has been cancelled. Satisfies the Writer
// function signature.
func (f *FanoutWriter) Write(e *loggregator_v2.Envelope) {
	select {
	case <-f.ctx.Done():
	case f.ch <- e:
	}
}

func (f *FanoutWriter) writeWorker() {
	defer f.wg.Done()
	for env := range f.ch {
		f.w(env)
	}
}
