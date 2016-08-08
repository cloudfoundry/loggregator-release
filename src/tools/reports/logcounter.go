package reports

type Errors struct {
	OneThousandEight int `json:"1008"`
	Misc             int `json:"other"`
}

type MessageCount struct {
	App   string `json:"app"`
	Total int    `json:"total"`
	Max   int    `json:"max"`
}

type LogCount struct {
	Errors   Errors                   `json:"errors"`
	Messages map[string]*MessageCount `json:"messages"`
}

func (r *LogCount) Add(report LogCount) {
	r.Errors.OneThousandEight += report.Errors.OneThousandEight
	r.Errors.Misc += report.Errors.Misc
	for id, count := range report.Messages {
		fullCount, ok := r.Messages[id]
		if !ok {
			r.Messages[id] = count
			continue
		}
		if count.Max > fullCount.Max {
			fullCount.Max = count.Max
		}
		fullCount.Total += count.Total
	}
}
