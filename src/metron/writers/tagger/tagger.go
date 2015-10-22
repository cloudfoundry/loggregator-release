package tagger

import (
	"strconv"

	"metron/writers"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	"github.com/pivotal-golang/localip"
)

type Tagger struct {
	deploymentName string
	job            string
	index          string
	ip             string
	outputWriter   writers.EnvelopeWriter
}

func New(deploymentName string, job string, index uint, outputWriter writers.EnvelopeWriter) *Tagger {
	ip, _ := localip.LocalIP()
	return &Tagger{
		deploymentName: deploymentName,
		job:            job,
		index:          strconv.Itoa(int(index)),
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
