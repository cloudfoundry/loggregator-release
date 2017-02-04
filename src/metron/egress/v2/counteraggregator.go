package egress

import (
	"crypto/sha1"
	"fmt"
	"io"
	"sort"

	plumbing "plumbing/v2"
)

type counterID struct {
	name     string
	tagsHash string
}

type CounterAggregator struct {
	writer        Writer
	counterTotals map[counterID]uint64
}

func New(w Writer) *CounterAggregator {
	return &CounterAggregator{
		writer:        w,
		counterTotals: make(map[counterID]uint64),
	}
}

func (ca *CounterAggregator) Write(msg *plumbing.Envelope) error {
	if msg.GetCounter() != nil {
		if len(ca.counterTotals) > 10000 {
			ca.resetTotals()
		}

		id := counterID{
			name:     msg.GetCounter().Name,
			tagsHash: hashTags(msg.GetTags()),
		}

		ca.counterTotals[id] = ca.counterTotals[id] + msg.GetCounter().GetDelta()

		msg.GetCounter().Value = &plumbing.Counter_Total{
			Total: ca.counterTotals[id],
		}
	}

	return ca.writer.Write(msg)
}

func (ca *CounterAggregator) resetTotals() {
	ca.counterTotals = make(map[counterID]uint64)
}

func hashTags(tags map[string]*plumbing.Value) string {
	hash := ""
	elements := []mapElement{}
	for k, v := range tags {
		elements = append(elements, mapElement{k, v.String()})
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
