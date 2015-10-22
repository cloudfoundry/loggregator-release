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
	newEnvelope := *envelope

	newEnvelope.Deployment = proto.String(t.deploymentName)
	newEnvelope.Job = proto.String(t.job)
	newEnvelope.Index = proto.String(t.index)
	newEnvelope.Ip = proto.String(t.ip)

	t.outputWriter.Write(&newEnvelope)
}
