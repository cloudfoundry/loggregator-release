package egress

import (
	"errors"
	"fmt"
	"io"
	"log"
	v2 "plumbing/v2"

	"golang.org/x/net/context"
)

type Receiver interface {
	Receive(ctx context.Context, req *v2.EgressRequest) (rx func() (*v2.Envelope, error), err error)
}

type Server struct {
	receiver Receiver
}

func NewServer(r Receiver) *Server {
	return &Server{
		receiver: r,
	}
}

func (s *Server) Receiver(r *v2.EgressRequest, srv v2.Egress_ReceiverServer) error {
	if r.GetFilter() != nil &&
		r.GetFilter().SourceId == "" &&
		r.GetFilter().Message != nil {
		return errors.New("invalid request: cannot have type filter without source id")
	}

	rx, err := s.receiver.Receive(srv.Context(), r)
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
