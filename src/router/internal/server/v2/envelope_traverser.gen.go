package v2

import (
	"hash/crc64"

	loggregator_v2 "code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/go-pubsub"
)

func envelopeTraverserTraverse(data interface{}) pubsub.Paths {
	return _SourceId(data)
}

func done(data interface{}) pubsub.Paths {
	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		return 0, nil, false
	})
}

func hashBool(data bool) uint64 {
	// 0 is reserved
	if data {
		return 2
	}
	return 1
}

func hashUint64(data uint64) uint64 {
	// 0 is reserved
	if data == 0 {
		return 1
	}

	return data
}

var tableECMA = crc64.MakeTable(crc64.ECMA)

func _SourceId(data interface{}) pubsub.Paths {

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 0,
				pubsub.TreeTraverser(func(data interface{}) pubsub.Paths {
					return ___Message
				}), true
		case 1:

			return hashUint64(crc64.Checksum([]byte(data.(*loggregator_v2.Envelope).SourceId), tableECMA)),
				pubsub.TreeTraverser(func(data interface{}) pubsub.Paths {
					return ___Message
				}), true
		default:
			return 0, nil, false
		}
	})
}

func ___Message(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
	switch idx {

	case 0:
		switch data.(*loggregator_v2.Envelope).Message.(type) {
		case *loggregator_v2.Envelope_Log:
			return 4, _Message_Envelope_Log_Log, true

		case *loggregator_v2.Envelope_Counter:
			return 1, _Message_Envelope_Counter_Counter, true

		case *loggregator_v2.Envelope_Gauge:
			return 3, _Message_Envelope_Gauge_Gauge, true

		case *loggregator_v2.Envelope_Timer:
			return 5, _Message_Envelope_Timer_Timer, true

		case *loggregator_v2.Envelope_Event:
			return 2, _Message_Envelope_Event_Event, true

		default:
			return 0, pubsub.TreeTraverser(done), true
		}

	default:
		return 0, nil, false
	}
}

func _Message_Envelope_Log(data interface{}) pubsub.Paths {

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 1, pubsub.TreeTraverser(done), true
		default:
			return 0, nil, false
		}
	})
}

func ___Message_Envelope_Log_Log(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
	switch idx {

	case 0:

		if data.(*loggregator_v2.Envelope).Message.(*loggregator_v2.Envelope_Log).Log == nil {
			return 0, pubsub.TreeTraverser(done), true
		}

		// Empty field name (data.(*loggregator_v2.Envelope).Message.(*loggregator_v2.Envelope_Log).Log)
		return 1, pubsub.TreeTraverser(done), true

	default:
		return 0, nil, false
	}
}

func _Message_Envelope_Log_Log(data interface{}) pubsub.Paths {

	if data.(*loggregator_v2.Envelope).Message.(*loggregator_v2.Envelope_Log).Log == nil {
		return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
			switch idx {
			case 0:
				return 0, pubsub.TreeTraverser(done), true
			default:
				return 0, nil, false
			}
		})
	}

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 1, pubsub.TreeTraverser(done), true
		default:
			return 0, nil, false
		}
	})
}

func _Message_Envelope_Counter(data interface{}) pubsub.Paths {

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 1, pubsub.TreeTraverser(done), true
		default:
			return 0, nil, false
		}
	})
}

func ___Message_Envelope_Counter_Counter(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
	switch idx {

	case 0:

		if data.(*loggregator_v2.Envelope).Message.(*loggregator_v2.Envelope_Counter).Counter == nil {
			return 0, pubsub.TreeTraverser(done), true
		}

		return 1, pubsub.TreeTraverser(_Message_Envelope_Counter_Counter_Name), true

	default:
		return 0, nil, false
	}
}

func _Message_Envelope_Counter_Counter(data interface{}) pubsub.Paths {

	if data.(*loggregator_v2.Envelope).Message.(*loggregator_v2.Envelope_Counter).Counter == nil {
		return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
			switch idx {
			case 0:
				return 0, pubsub.TreeTraverser(done), true
			default:
				return 0, nil, false
			}
		})
	}

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 1, pubsub.TreeTraverser(_Message_Envelope_Counter_Counter_Name), true
		default:
			return 0, nil, false
		}
	})
}

func _Message_Envelope_Counter_Counter_Name(data interface{}) pubsub.Paths {

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 0, pubsub.TreeTraverser(done), true
		case 1:

			return hashUint64(crc64.Checksum([]byte(data.(*loggregator_v2.Envelope).Message.(*loggregator_v2.Envelope_Counter).Counter.Name), tableECMA)), pubsub.TreeTraverser(done), true
		default:
			return 0, nil, false
		}
	})
}

func _Message_Envelope_Gauge(data interface{}) pubsub.Paths {

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 1, pubsub.TreeTraverser(done), true
		default:
			return 0, nil, false
		}
	})
}

func ___Message_Envelope_Gauge_Gauge(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
	switch idx {

	case 0:

		if data.(*loggregator_v2.Envelope).Message.(*loggregator_v2.Envelope_Gauge).Gauge == nil {
			return 0, pubsub.TreeTraverser(done), true
		}

		return 1, pubsub.TreeTraverser(_Message_Envelope_Gauge_Gauge_Metrics), true

	default:
		return 0, nil, false
	}
}

func _Message_Envelope_Gauge_Gauge(data interface{}) pubsub.Paths {

	if data.(*loggregator_v2.Envelope).Message.(*loggregator_v2.Envelope_Gauge).Gauge == nil {
		return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
			switch idx {
			case 0:
				return 0, pubsub.TreeTraverser(done), true
			default:
				return 0, nil, false
			}
		})
	}

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 1, pubsub.TreeTraverser(_Message_Envelope_Gauge_Gauge_Metrics), true
		default:
			return 0, nil, false
		}
	})
}

func _Message_Envelope_Gauge_Gauge_Metrics(data interface{}) pubsub.Paths {

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 0, pubsub.TreeTraverser(done), true
		case 1:

			var total uint64
			for x := range data.(*loggregator_v2.Envelope).Message.(*loggregator_v2.Envelope_Gauge).Gauge.Metrics {
				total += hashUint64(crc64.Checksum([]byte(x), tableECMA))
			}
			return hashUint64(total), pubsub.TreeTraverser(done), true
		default:
			return 0, nil, false
		}
	})
}

func _Message_Envelope_Timer(data interface{}) pubsub.Paths {

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 1, pubsub.TreeTraverser(done), true
		default:
			return 0, nil, false
		}
	})
}

func ___Message_Envelope_Timer_Timer(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
	switch idx {

	case 0:

		if data.(*loggregator_v2.Envelope).Message.(*loggregator_v2.Envelope_Timer).Timer == nil {
			return 0, pubsub.TreeTraverser(done), true
		}

		// Empty field name (data.(*loggregator_v2.Envelope).Message.(*loggregator_v2.Envelope_Timer).Timer)
		return 1, pubsub.TreeTraverser(done), true

	default:
		return 0, nil, false
	}
}

func _Message_Envelope_Timer_Timer(data interface{}) pubsub.Paths {

	if data.(*loggregator_v2.Envelope).Message.(*loggregator_v2.Envelope_Timer).Timer == nil {
		return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
			switch idx {
			case 0:
				return 0, pubsub.TreeTraverser(done), true
			default:
				return 0, nil, false
			}
		})
	}

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 1, pubsub.TreeTraverser(done), true
		default:
			return 0, nil, false
		}
	})
}

func _Message_Envelope_Event(data interface{}) pubsub.Paths {

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 1, pubsub.TreeTraverser(done), true
		default:
			return 0, nil, false
		}
	})
}

func ___Message_Envelope_Event_Event(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
	switch idx {

	case 0:

		if data.(*loggregator_v2.Envelope).Message.(*loggregator_v2.Envelope_Event).Event == nil {
			return 0, pubsub.TreeTraverser(done), true
		}

		return 1, pubsub.TreeTraverser(_Message_Envelope_Event_Event_Title), true

	default:
		return 0, nil, false
	}
}

func _Message_Envelope_Event_Event(data interface{}) pubsub.Paths {

	if data.(*loggregator_v2.Envelope).Message.(*loggregator_v2.Envelope_Event).Event == nil {
		return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
			switch idx {
			case 0:
				return 0, pubsub.TreeTraverser(done), true
			default:
				return 0, nil, false
			}
		})
	}

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 1, pubsub.TreeTraverser(_Message_Envelope_Event_Event_Title), true
		default:
			return 0, nil, false
		}
	})
}

func _Message_Envelope_Event_Event_Title(data interface{}) pubsub.Paths {

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 0, pubsub.TreeTraverser(_Message_Envelope_Event_Event_Body), true
		case 1:

			return hashUint64(crc64.Checksum([]byte(data.(*loggregator_v2.Envelope).Message.(*loggregator_v2.Envelope_Event).Event.Title), tableECMA)), pubsub.TreeTraverser(_Message_Envelope_Event_Event_Body), true
		default:
			return 0, nil, false
		}
	})
}

func _Message_Envelope_Event_Event_Body(data interface{}) pubsub.Paths {

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 0, pubsub.TreeTraverser(done), true
		case 1:

			return hashUint64(crc64.Checksum([]byte(data.(*loggregator_v2.Envelope).Message.(*loggregator_v2.Envelope_Event).Event.Body), tableECMA)), pubsub.TreeTraverser(done), true
		default:
			return 0, nil, false
		}
	})
}

type EnvelopeFilter struct {
	SourceId                 *string
	Message_Envelope_Log     *Envelope_LogFilter
	Message_Envelope_Counter *Envelope_CounterFilter
	Message_Envelope_Gauge   *Envelope_GaugeFilter
	Message_Envelope_Timer   *Envelope_TimerFilter
	Message_Envelope_Event   *Envelope_EventFilter
}

type Envelope_LogFilter struct {
	Log *LogFilter
}

type LogFilter struct {
}

type Envelope_CounterFilter struct {
	Counter *CounterFilter
}

type CounterFilter struct {
	Name *string
}

type Envelope_GaugeFilter struct {
	Gauge *GaugeFilter
}

type GaugeFilter struct {
	Metrics []string
}

type Envelope_TimerFilter struct {
	Timer *TimerFilter
}

type TimerFilter struct {
}

type Envelope_EventFilter struct {
	Event *EventFilter
}

type EventFilter struct {
	Title *string
	Body  *string
}

func envelopeTraverserCreatePath(f *EnvelopeFilter) []uint64 {
	if f == nil {
		return nil
	}
	var path []uint64

	var count int
	if f.Message_Envelope_Log != nil {
		count++
	}

	if f.Message_Envelope_Counter != nil {
		count++
	}

	if f.Message_Envelope_Gauge != nil {
		count++
	}

	if f.Message_Envelope_Timer != nil {
		count++
	}

	if f.Message_Envelope_Event != nil {
		count++
	}

	if count > 1 {
		panic("Only one field can be set")
	}

	if f.SourceId != nil {

		path = append(path, hashUint64(crc64.Checksum([]byte(*f.SourceId), tableECMA)))
	} else {
		path = append(path, 0)
	}

	path = append(path, createPath__Message_Envelope_Counter(f.Message_Envelope_Counter)...)

	path = append(path, createPath__Message_Envelope_Event(f.Message_Envelope_Event)...)

	path = append(path, createPath__Message_Envelope_Gauge(f.Message_Envelope_Gauge)...)

	path = append(path, createPath__Message_Envelope_Log(f.Message_Envelope_Log)...)

	path = append(path, createPath__Message_Envelope_Timer(f.Message_Envelope_Timer)...)

	for i := len(path) - 1; i >= 1; i-- {
		if path[i] != 0 {
			break
		}
		path = path[:i]
	}

	return path
}

func createPath__Message_Envelope_Counter(f *Envelope_CounterFilter) []uint64 {
	if f == nil {
		return nil
	}
	var path []uint64

	path = append(path, 1)

	var count int
	if f.Counter != nil {
		count++
	}

	if count > 1 {
		panic("Only one field can be set")
	}

	path = append(path, createPath__Envelope_Counter_Counter(f.Counter)...)

	return path
}

func createPath__Envelope_Counter_Counter(f *CounterFilter) []uint64 {
	if f == nil {
		return nil
	}
	var path []uint64

	path = append(path, 1)

	var count int
	if count > 1 {
		panic("Only one field can be set")
	}

	if f.Name != nil {

		path = append(path, hashUint64(crc64.Checksum([]byte(*f.Name), tableECMA)))
	} else {
		path = append(path, 0)
	}

	return path
}

func createPath__Message_Envelope_Event(f *Envelope_EventFilter) []uint64 {
	if f == nil {
		return nil
	}
	var path []uint64

	path = append(path, 2)

	var count int
	if f.Event != nil {
		count++
	}

	if count > 1 {
		panic("Only one field can be set")
	}

	path = append(path, createPath__Envelope_Event_Event(f.Event)...)

	return path
}

func createPath__Envelope_Event_Event(f *EventFilter) []uint64 {
	if f == nil {
		return nil
	}
	var path []uint64

	path = append(path, 1)

	var count int
	if count > 1 {
		panic("Only one field can be set")
	}

	if f.Title != nil {

		path = append(path, hashUint64(crc64.Checksum([]byte(*f.Title), tableECMA)))
	} else {
		path = append(path, 0)
	}

	if f.Body != nil {

		path = append(path, hashUint64(crc64.Checksum([]byte(*f.Body), tableECMA)))
	} else {
		path = append(path, 0)
	}

	return path
}

func createPath__Message_Envelope_Gauge(f *Envelope_GaugeFilter) []uint64 {
	if f == nil {
		return nil
	}
	var path []uint64

	path = append(path, 3)

	var count int
	if f.Gauge != nil {
		count++
	}

	if count > 1 {
		panic("Only one field can be set")
	}

	path = append(path, createPath__Envelope_Gauge_Gauge(f.Gauge)...)

	return path
}

func createPath__Envelope_Gauge_Gauge(f *GaugeFilter) []uint64 {
	if f == nil {
		return nil
	}
	var path []uint64

	path = append(path, 1)

	var count int
	if count > 1 {
		panic("Only one field can be set")
	}

	if f.Metrics != nil {

		var total uint64
		for _, x := range f.Metrics {
			total += hashUint64(crc64.Checksum([]byte(x), tableECMA))
		}
		path = append(path, hashUint64(total))
	} else {
		path = append(path, 0)
	}

	return path
}

func createPath__Message_Envelope_Log(f *Envelope_LogFilter) []uint64 {
	if f == nil {
		return nil
	}
	var path []uint64

	path = append(path, 4)

	var count int
	if f.Log != nil {
		count++
	}

	if count > 1 {
		panic("Only one field can be set")
	}

	path = append(path, createPath__Envelope_Log_Log(f.Log)...)

	return path
}

func createPath__Envelope_Log_Log(f *LogFilter) []uint64 {
	if f == nil {
		return nil
	}
	var path []uint64

	path = append(path, 1)

	var count int
	if count > 1 {
		panic("Only one field can be set")
	}

	return path
}

func createPath__Message_Envelope_Timer(f *Envelope_TimerFilter) []uint64 {
	if f == nil {
		return nil
	}
	var path []uint64

	path = append(path, 5)

	var count int
	if f.Timer != nil {
		count++
	}

	if count > 1 {
		panic("Only one field can be set")
	}

	path = append(path, createPath__Envelope_Timer_Timer(f.Timer)...)

	return path
}

func createPath__Envelope_Timer_Timer(f *TimerFilter) []uint64 {
	if f == nil {
		return nil
	}
	var path []uint64

	path = append(path, 1)

	var count int
	if count > 1 {
		panic("Only one field can be set")
	}

	return path
}
