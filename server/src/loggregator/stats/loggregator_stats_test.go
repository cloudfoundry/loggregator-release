package stats

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"instrumentor"
	"encoding/json"
)

type FakeInstrumentable struct {
	data []instrumentor.PropVal
}

func (i FakeInstrumentable) DumpData() []instrumentor.PropVal {
	return i.data
}

func TestSerializesDataFromEachInstrumentable(t *testing.T) {
	expected, err := json.Marshal(map[string]string{
		"Clients Connected": "876",
		"Messages Sent": "1000",
	})

	assert.NoError(t, err)

	data := []instrumentor.PropVal{
		instrumentor.PropVal{"Clients Connected", "876"},
	}
	i1 := &FakeInstrumentable{data}

	data = []instrumentor.PropVal{
		instrumentor.PropVal{"Messages Sent", "1000"},
	}
	i2 := &FakeInstrumentable{data}

	instrumentables := []instrumentor.Instrumentable{i1, i2}
	stats := &LoggregatorStats{instrumentables}

	actual, err := stats.MarshalJSON()

	assert.NoError(t, err)

	assert.Equal(t, expected, actual)
}
