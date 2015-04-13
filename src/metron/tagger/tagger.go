package tagger

import (
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/gogo/protobuf/proto"
	"github.com/pivotal-golang/localip"
	"strconv"
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
		newEnvelope.Tags = append(envelope.Tags,
			&events.Tag{
				Key:   proto.String("deployment"),
				Value: proto.String(t.deploymentName),
			},
			&events.Tag{
				Key:   proto.String("job"),
				Value: proto.String(t.job),
			},
			&events.Tag{
				Key:   proto.String("index"),
				Value: proto.String(strconv.Itoa(int(t.index))),
			},
			&events.Tag{
				Key:   proto.String("ip"),
				Value: proto.String(ip),
			})

		outputChan <- &newEnvelope
	}
}
