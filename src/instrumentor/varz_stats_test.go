package instrumentor

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
)

type FakeInstrumentable struct {
	data []PropVal
}

func (i FakeInstrumentable) DumpData() []PropVal {
	return i.data
}

func TestSerializesDataFromEachInstrumentable(t *testing.T) {
	expected, err := json.Marshal(map[string]string{
		"Clients Connected": "876",
		"Messages Sent":     "1000",
	})

	assert.NoError(t, err)

	data := []PropVal{
		PropVal{"Clients Connected", "876"},
	}
	i1 := &FakeInstrumentable{data}

	data = []PropVal{
		PropVal{"Messages Sent", "1000"},
	}
	i2 := &FakeInstrumentable{data}

	instrumentables := []Instrumentable{i1, i2}
	stats := &VarzStats{instrumentables}

	actual, err := stats.MarshalJSON()

	assert.NoError(t, err)

	assert.Equal(t, expected, actual)
}
