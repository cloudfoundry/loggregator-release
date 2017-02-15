package egress

import (
	"fmt"
	"io"
	"log"
	v2 "plumbing/v2"

	"golang.org/x/net/context"
)

type Subscriber interface {
	Subscribe(ctx context.Context, req *v2.EgressRequest) (rx func() (*v2.Envelope, error), err error)
}

type Server struct {
	subscriber Subscriber
}

func NewServer(s Subscriber) *Server {
	return &Server{
		subscriber: s,
	}
}

func (s *Server) Receiver(r *v2.EgressRequest, srv v2.Egress_ReceiverServer) error {
	rx, err := s.subscriber.Subscribe(srv.Context(), r)
	if err != nil {
		log.Printf("Unable to setup subscription: %s", err)
		return fmt.Errorf("unable to setup subscription")
	}

	for {
		e, err := rx()
		if err == io.EOF {
			return io.EOF
		}

		if err != nil {
			log.Printf("Subscribe error: %s", err)
			return io.ErrUnexpectedEOF
		}

		if err := srv.Send(e); err != nil {
			log.Printf("Send error: %s", err)
			return io.ErrUnexpectedEOF
		}
	}
}
