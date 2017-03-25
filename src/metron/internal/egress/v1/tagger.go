package v1

import (
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

type Tagger struct {
	deploymentName string
	job            string
	index          string
	ip             string
	outputWriter   EnvelopeWriter
}

func NewTagger(deploymentName, job, index, ip string, outputWriter EnvelopeWriter) *Tagger {
	return &Tagger{
		deploymentName: deploymentName,
		job:            job,
		index:          index,
		ip:             ip,
		outputWriter:   outputWriter,
	}
}

func (t *Tagger) Write(envelope *events.Envelope) {
	newEnvelope := envelope
	t.setDefaultTags(newEnvelope)
	t.outputWriter.Write(newEnvelope)
}

func (t *Tagger) setDefaultTags(envelope *events.Envelope) {
	if envelope.Deployment == nil {
		envelope.Deployment = proto.String(t.deploymentName)
	}
	if envelope.Job == nil {
		envelope.Job = proto.String(t.job)
	}
	if envelope.Index == nil {
		envelope.Index = proto.String(t.index)
	}
	if envelope.Ip == nil {
		envelope.Ip = proto.String(t.ip)
	}
}
