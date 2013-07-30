package stats

import (
	"encoding/json"
	"instrumentor"
)

func NewLoggregatorStats(is []instrumentor.Instrumentable) *LoggregatorStats{
	return &LoggregatorStats{is}
}

type LoggregatorStats struct {
	instrumentables []instrumentor.Instrumentable
}

func (s *LoggregatorStats) MarshalJSON() ([]byte, error) {
	acc := make(map[string]string)

	for _, i := range s.instrumentables {
		for _, prop := range i.DumpData() {
			acc[prop.Property] = prop.Value
		}
	}
	return json.Marshal(acc)
}
