package tagger

import (
	"strconv"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	"github.com/pivotal-golang/localip"
	"metron/envelopewriter"
)

type Tagger struct {
	deploymentName string
	job            string
	index          uint
	ip			   string
	outputWriter   envelopewriter.EnvelopeWriter
}

func New(deploymentName string, job string, index uint, outputWriter envelopewriter.EnvelopeWriter) *Tagger {
	ip, _ := localip.LocalIP()
	return &Tagger{
		deploymentName: deploymentName,
		job:            job,
		index:          index,
		ip: 			ip,
		outputWriter:   outputWriter,
	}
}

func (t *Tagger) Write(envelope *events.Envelope) {
	newEnvelope := *envelope

	newEnvelope.Deployment = proto.String(t.deploymentName)
	newEnvelope.Job = proto.String(t.job)
	newEnvelope.Index = proto.String(strconv.Itoa(int(t.index)))
	newEnvelope.Ip = proto.String(t.ip)

	t.outputWriter.Write(&newEnvelope)
}
