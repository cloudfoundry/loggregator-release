package instrumentor

import "encoding/json"

func NewVarzStats(is []Instrumentable) *VarzStats {
	return &VarzStats{is}
}

type VarzStats struct {
	instrumentables []Instrumentable
}

func (s *VarzStats) MarshalJSON() ([]byte, error) {
	acc := make(map[string]string)

	for _, i := range s.instrumentables {
		for _, prop := range i.DumpData() {
			acc[prop.Property] = prop.Value
		}
	}
	return json.Marshal(acc)
}
