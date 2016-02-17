package picker

import (
	"errors"

	"math/rand"

	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

//go:generate hel --type WeightedByteWriter --output mock_weighted_writer_test.go

type WeightedByteWriter interface {
	Write(message []byte) (sentLength int, err error)
	Weight() int
}

type Picker struct {
	writers       []WeightedByteWriter
	logger        *gosteno.Logger
	defaultWriter WeightedByteWriter
}

func New(logger *gosteno.Logger, defaultWriter WeightedByteWriter, byteWriters ...WeightedByteWriter) (*Picker, error) {
	if len(byteWriters) == 0 {
		return nil, errors.New("No writers available")
	}

	return &Picker{
		logger:        logger,
		writers:       byteWriters,
		defaultWriter: defaultWriter,
	}, nil
}

func (p *Picker) Write(envelope *events.Envelope) {

	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		p.logger.Errorf("marshalling error: %v", err)
		metrics.BatchIncrementCounter("dropsondeMarshaller.marshalErrors")
	}
	writer := p.pick()
	writer.Write(envelopeBytes)
}

func (p *Picker) pick() WeightedByteWriter {
	var totalWeight = 0

	cumulativeWeights := make([]int, len(p.writers))
	for i, writer := range p.writers {
		weight := writer.Weight()
		totalWeight += weight
		cumulativeWeights[i] = totalWeight
	}

	if totalWeight == 0 {
		return p.defaultWriter
	}

	randomWeight := rand.Intn(totalWeight) + 1
	for i, weight := range cumulativeWeights {
		if randomWeight <= weight {
			return p.writers[i]
		}
	}

	return p.defaultWriter
}
