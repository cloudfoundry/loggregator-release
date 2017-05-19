package egress

import (
	"errors"
	"fmt"
	"io"
	"log"

	"diodes"
	"metricemitter"
	v2 "plumbing/v2"

	gendiodes "github.com/cloudfoundry/diodes"
	"golang.org/x/net/context"
)

type Receiver interface {
	Receive(ctx context.Context, req *v2.EgressRequest) (rx func() (*v2.Envelope, error), err error)
}

type Server struct {
	receiver     Receiver
	egressMetric *metricemitter.CounterMetric
}

func NewServer(r Receiver, m metricemitter.MetricClient) *Server {
	egressMetric := m.NewCounterMetric(
		"egress",
		metricemitter.WithTags(map[string]string{
			"protocol": "grpc",
		}),
		metricemitter.WithVersion(2, 0),
	)

	return &Server{
		receiver:     r,
		egressMetric: egressMetric,
	}
}

func (s *Server) Receiver(r *v2.EgressRequest, srv v2.Egress_ReceiverServer) error {
	if r.GetFilter() != nil &&
		r.GetFilter().SourceId == "" &&
		r.GetFilter().Message != nil {
		return errors.New("invalid request: cannot have type filter without source id")
	}

	ctx, cancel := context.WithCancel(srv.Context())
	rx, err := s.receiver.Receive(ctx, r)
	if err != nil {
		log.Printf("Unable to setup subscription: %s", err)
		return fmt.Errorf("unable to setup subscription")
	}

	alerter := gendiodes.AlertFunc(func(missed int) {
		fmt.Println("DROPPED")
	})

	buffer := diodes.NewOneToOneEnvelopeV2(100, alerter, gendiodes.WithPollingContext(ctx))

	go func() {
		defer cancel()
		for {
			e, err := rx()
			if err == io.EOF {
				break
			}

			if err != nil {
				log.Printf("Subscribe error: %s", err)
				break
			}

			buffer.Set(e)
		}
	}()

	for {
		data := buffer.Next()
		if data == nil {
			return nil
		}

		if err := srv.Send(data); err != nil {
			log.Printf("Send error: %s", err)
			return io.ErrUnexpectedEOF
		}

		// metric-documentation-v2: (egress) Number of v2 envelopes sent to RLP
		// consumers.
		s.egressMetric.Increment(1)
	}
}
