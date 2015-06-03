package tagger

import (
	"strconv"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	"github.com/pivotal-golang/localip"
)

type Tagger struct {
	deploymentName string
	job            string
	index          uint
}

func New(deploymentName string, job string, index uint) *Tagger {
	return &Tagger{
		deploymentName: deploymentName,
		job:            job,
		index:          index,
	}
}

func (t *Tagger) Run(inputChan <-chan *events.Envelope, outputChan chan<- *events.Envelope) {
	ip, _ := localip.LocalIP()
	for envelope := range inputChan {
		newEnvelope := *envelope

		newEnvelope.Deployment = proto.String(t.deploymentName)
		newEnvelope.Job = proto.String(t.job)
		newEnvelope.Index = proto.String(strconv.Itoa(int(t.index)))
		newEnvelope.Ip = proto.String(ip)

		outputChan <- &newEnvelope
	}
}
