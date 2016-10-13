package grpcconnector

import (
	"fmt"
	"log"
	"plumbing"
	"time"
)

type combiner struct {
	rxs        []Receiver
	mainOutput chan []byte
	errs       chan error
	batcher    MetaMetricBatcher
	killAfter  time.Duration
}

func startCombiner(rxs []Receiver, batcher MetaMetricBatcher, killAfter time.Duration, bufferSize int) *combiner {
	c := &combiner{
		rxs:        rxs,
		batcher:    batcher,
		mainOutput: make(chan []byte, bufferSize),
		errs:       make(chan error, 10),
		killAfter:  killAfter,
	}

	c.start()
	return c
}

func (c *combiner) Recv() (*plumbing.Response, error) {
	select {
	case err := <-c.errs:
		return nil, err
	case payload := <-c.mainOutput:
		return &plumbing.Response{
			Payload: payload,
		}, nil
	}
}

func (c *combiner) start() {
	for _, rx := range c.rxs {
		go c.readFromReceiver(rx)
	}
}

func (c *combiner) readFromReceiver(rx Receiver) {
	timer := time.NewTimer(time.Second)
	timer.Stop()
	for {
		resp, err := rx.Recv()
		if err != nil {
			c.writeError(err)
			return
		}

		c.batcher.BatchCounter("listeners.receivedEnvelopes").
			SetTag("protocol", "grpc").
			Increment()

		timer.Reset(c.killAfter)
		select {
		case c.mainOutput <- resp.Payload:
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
			log.Println("Slow connection: Dropping connection")

			c.batcher.BatchCounter("listeners.slowConsumer").
				SetTag("protocol", "grpc").
				Increment()

			c.writeError(fmt.Errorf("Slow consumer"))
			return
		}
	}
}

func (c *combiner) writeError(err error) {
	select {
	case c.errs <- err:
	default:
		log.Printf("Combiner unable to write err (channel full): %s", err)
	}
}
