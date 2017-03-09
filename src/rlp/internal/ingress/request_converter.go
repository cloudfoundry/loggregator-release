package ingress

import (
	"plumbing"
	v2 "plumbing/v2"
)

type requestConverter struct{}

func NewRequestConverter() RequestConverter {
	return requestConverter{}
}

func (r requestConverter) Convert(v2req *v2.EgressRequest) *plumbing.SubscriptionRequest {
	return &plumbing.SubscriptionRequest{
		ShardID: v2req.ShardId,
		Filter:  r.convertFilter(v2req.GetFilter()),
	}
}

func (r requestConverter) convertFilter(v2filter *v2.Filter) *plumbing.Filter {
	if v2filter == nil {
		return nil
	}

	f := &plumbing.Filter{
		AppID: v2filter.SourceId,
	}

	switch v2filter.GetMessage().(type) {
	case *v2.Filter_Log:
		f.Message = &plumbing.Filter_Log{
			&plumbing.LogFilter{},
		}
	case *v2.Filter_Counter:
		f.Message = &plumbing.Filter_Counter{
			&plumbing.CounterFilter{},
		}
	case *v2.Filter_Gauge:
		f.Message = &plumbing.Filter_Gauge{
			&plumbing.GaugeFilter{},
		}
	case *v2.Filter_Timer:
		f.Message = &plumbing.Filter_Timer{
			&plumbing.TimerFilter{},
		}
	}

	return f
}
