package v1

import (
	"crypto/sha1"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/sonde-go/events"
)

var MaxTTL = time.Minute

type MessageAggregator struct {
	mu            sync.Mutex
	counterTotals map[counterID]uint64
	outputWriter  EnvelopeWriter
}

func NewAggregator(outputWriter EnvelopeWriter) *MessageAggregator {
	return &MessageAggregator{
		outputWriter:  outputWriter,
		counterTotals: make(map[counterID]uint64),
	}
}

func (m *MessageAggregator) Write(envelope *events.Envelope) {
	if envelope.GetEventType() == events.Envelope_CounterEvent {
		envelope = m.handleCounter(envelope)
	}
	m.outputWriter.Write(envelope)
}

func (m *MessageAggregator) handleCounter(envelope *events.Envelope) *events.Envelope {
	// metric-documentation-v1: (MessageAggregator.counterEventReceived) Total number of
	// counter events received by the message aggregator.
	metrics.BatchIncrementCounter("MessageAggregator.counterEventReceived")

	countID := counterID{
		name:     envelope.GetCounterEvent().GetName(),
		origin:   envelope.GetOrigin(),
		tagsHash: hashTags(envelope.Tags),
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	newVal := m.counterTotals[countID] + envelope.GetCounterEvent().GetDelta()
	m.counterTotals[countID] = newVal

	envelope.GetCounterEvent().Total = &newVal
	return envelope
}

func hashTags(tags map[string]string) string {
	hash := ""
	elements := []mapElement{}
	for k, v := range tags {
		elements = append(elements, mapElement{k, v})
	}
	sort.Sort(byKey(elements))
	for _, element := range elements {
		kHash, vHash := sha1.New(), sha1.New()
		io.WriteString(kHash, element.k)
		io.WriteString(vHash, element.v)
		hash += fmt.Sprintf("%x%x", kHash.Sum(nil), vHash.Sum(nil))
	}
	return hash
}

type byKey []mapElement

func (a byKey) Len() int           { return len(a) }
func (a byKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byKey) Less(i, j int) bool { return a[i].k < a[j].k }

type mapElement struct {
	k, v string
}

type counterID struct {
	origin   string
	name     string
	tagsHash string
}

type eventID struct {
	requestID string
	peerType  events.PeerType
}
