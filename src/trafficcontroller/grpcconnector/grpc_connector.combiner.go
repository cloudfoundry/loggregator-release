package grpcconnector

import "plumbing"

type combiner struct {
	rxs        []Receiver
	mainOutput chan []byte
	errs       chan error
}

func startCombiner(rxs []Receiver) *combiner {
	c := &combiner{
		rxs:        rxs,
		mainOutput: make(chan []byte, 1000),
		errs:       make(chan error, 1000),
	}

	c.start()
	return c
}

func (c *combiner) Recv() (*plumbing.Response, error) {
	select {
	case payload := <-c.mainOutput:
		return &plumbing.Response{
			Payload: payload,
		}, nil
	case err := <-c.errs:
		return nil, err
	}
}

func (c *combiner) start() {
	for _, rx := range c.rxs {
		go func(r Receiver) {
			for {
				resp, err := r.Recv()
				if err != nil {
					c.errs <- err
					return
				}

				c.mainOutput <- resp.Payload
			}
		}(rx)
	}
}
