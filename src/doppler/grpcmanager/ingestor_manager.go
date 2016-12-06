package grpcmanager

import (
	"io"
	"plumbing"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

type IngestorManager struct {
	sender MessageSender
}
type MessageSender chan *events.Envelope
type IngestorGRPCServer interface {
	plumbing.DopplerIngestor_PusherServer
}

func NewIngestor(sender MessageSender) *IngestorManager {
	return &IngestorManager{
		sender: sender,
	}
}

func (i *IngestorManager) Pusher(pusher plumbing.DopplerIngestor_PusherServer) error {
	for {
		envelopeData, err := pusher.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		env := &events.Envelope{}
		err = proto.Unmarshal(envelopeData.Payload, env)
		if err != nil {
			continue
		}
		i.sender <- env
	}
	return nil
}
