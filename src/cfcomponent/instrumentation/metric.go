package instrumentation

type Metric struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}
