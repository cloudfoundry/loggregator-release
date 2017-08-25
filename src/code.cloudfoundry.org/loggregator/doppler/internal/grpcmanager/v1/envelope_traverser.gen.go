package v1

import (
	"hash/crc64"

	"github.com/apoydence/pubsub"
)

func envelopeTraverserTraverse(data interface{}) pubsub.Paths {
	return _AppID(data)
}

func done(data interface{}) pubsub.Paths {
	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		return 0, nil, false
	})
}

func hashBool(data bool) uint64 {
	if data {
		return 1
	}
	return 0
}

var tableECMA = crc64.MakeTable(crc64.ECMA)

func _AppID(data interface{}) pubsub.Paths {

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 0,
				pubsub.TreeTraverser(func(data interface{}) pubsub.Paths {
					return __Envelope
				}), true
		case 1:
			return crc64.Checksum([]byte(data.(*DataWrapper).AppID), tableECMA),
				pubsub.TreeTraverser(func(data interface{}) pubsub.Paths {
					return __Envelope
				}), true
		default:
			return 0, nil, false
		}
	})
}

func __Envelope(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
	switch idx {

	case 0:

		if data.(*DataWrapper).Envelope == nil {
			return 0, pubsub.TreeTraverser(done), true
		}

		return 1, pubsub.TreeTraverser(_Envelope_Ip), true

	default:
		return 0, nil, false
	}
}

func _Envelope(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope == nil {
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
			return 1, pubsub.TreeTraverser(_Envelope_Ip), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_Ip(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.Ip == nil {
		return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
			switch idx {
			case 0:
				return 0,
					pubsub.TreeTraverser(func(data interface{}) pubsub.Paths {
						return __HttpStartStop_LogMessage_ValueMetric_CounterEvent_Error_ContainerMetric
					}), true
			default:
				return 0, nil, false
			}
		})
	}

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 0,
				pubsub.TreeTraverser(func(data interface{}) pubsub.Paths {
					return __HttpStartStop_LogMessage_ValueMetric_CounterEvent_Error_ContainerMetric
				}), true
		case 1:
			return crc64.Checksum([]byte(*data.(*DataWrapper).Envelope.Ip), tableECMA),
				pubsub.TreeTraverser(func(data interface{}) pubsub.Paths {
					return __HttpStartStop_LogMessage_ValueMetric_CounterEvent_Error_ContainerMetric
				}), true
		default:
			return 0, nil, false
		}
	})
}

func __HttpStartStop_LogMessage_ValueMetric_CounterEvent_Error_ContainerMetric(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
	switch idx {

	case 0:

		if data.(*DataWrapper).Envelope.HttpStartStop == nil {
			return 0, pubsub.TreeTraverser(done), true
		}

		return 1, pubsub.TreeTraverser(_Envelope_HttpStartStop_RemoteAddress), true

	case 1:

		if data.(*DataWrapper).Envelope.LogMessage == nil {
			return 0, pubsub.TreeTraverser(done), true
		}

		return 2, pubsub.TreeTraverser(_Envelope_LogMessage_SourceInstance), true

	case 2:

		if data.(*DataWrapper).Envelope.ValueMetric == nil {
			return 0, pubsub.TreeTraverser(done), true
		}

		return 3, pubsub.TreeTraverser(_Envelope_ValueMetric_Name), true

	case 3:

		if data.(*DataWrapper).Envelope.CounterEvent == nil {
			return 0, pubsub.TreeTraverser(done), true
		}

		return 4, pubsub.TreeTraverser(_Envelope_CounterEvent_Name), true

	case 4:

		if data.(*DataWrapper).Envelope.Error == nil {
			return 0, pubsub.TreeTraverser(done), true
		}

		return 5, pubsub.TreeTraverser(_Envelope_Error_Source), true

	case 5:

		if data.(*DataWrapper).Envelope.ContainerMetric == nil {
			return 0, pubsub.TreeTraverser(done), true
		}

		return 6, pubsub.TreeTraverser(_Envelope_ContainerMetric_ApplicationId), true

	default:
		return 0, nil, false
	}
}

func _Envelope_HttpStartStop(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.HttpStartStop == nil {
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
			return 1, pubsub.TreeTraverser(_Envelope_HttpStartStop_RemoteAddress), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_HttpStartStop_RemoteAddress(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.HttpStartStop.RemoteAddress == nil {
		return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
			switch idx {
			case 0:
				return 0, pubsub.TreeTraverser(_Envelope_HttpStartStop_UserAgent), true
			default:
				return 0, nil, false
			}
		})
	}

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 0, pubsub.TreeTraverser(_Envelope_HttpStartStop_UserAgent), true
		case 1:
			return crc64.Checksum([]byte(*data.(*DataWrapper).Envelope.HttpStartStop.RemoteAddress), tableECMA), pubsub.TreeTraverser(_Envelope_HttpStartStop_UserAgent), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_HttpStartStop_UserAgent(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.HttpStartStop.UserAgent == nil {
		return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
			switch idx {
			case 0:
				return 0, pubsub.TreeTraverser(_Envelope_HttpStartStop_StatusCode), true
			default:
				return 0, nil, false
			}
		})
	}

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 0, pubsub.TreeTraverser(_Envelope_HttpStartStop_StatusCode), true
		case 1:
			return crc64.Checksum([]byte(*data.(*DataWrapper).Envelope.HttpStartStop.UserAgent), tableECMA), pubsub.TreeTraverser(_Envelope_HttpStartStop_StatusCode), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_HttpStartStop_StatusCode(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.HttpStartStop.StatusCode == nil {
		return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
			switch idx {
			case 0:
				return 0, pubsub.TreeTraverser(_Envelope_HttpStartStop_ContentLength), true
			default:
				return 0, nil, false
			}
		})
	}

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 0, pubsub.TreeTraverser(_Envelope_HttpStartStop_ContentLength), true
		case 1:
			return uint64(*data.(*DataWrapper).Envelope.HttpStartStop.StatusCode), pubsub.TreeTraverser(_Envelope_HttpStartStop_ContentLength), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_HttpStartStop_ContentLength(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.HttpStartStop.ContentLength == nil {
		return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
			switch idx {
			case 0:
				return 0, pubsub.TreeTraverser(_Envelope_HttpStartStop_InstanceIndex), true
			default:
				return 0, nil, false
			}
		})
	}

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 0, pubsub.TreeTraverser(_Envelope_HttpStartStop_InstanceIndex), true
		case 1:
			return uint64(*data.(*DataWrapper).Envelope.HttpStartStop.ContentLength), pubsub.TreeTraverser(_Envelope_HttpStartStop_InstanceIndex), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_HttpStartStop_InstanceIndex(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.HttpStartStop.InstanceIndex == nil {
		return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
			switch idx {
			case 0:
				return 0, pubsub.TreeTraverser(_Envelope_HttpStartStop_InstanceId), true
			default:
				return 0, nil, false
			}
		})
	}

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 0, pubsub.TreeTraverser(_Envelope_HttpStartStop_InstanceId), true
		case 1:
			return uint64(*data.(*DataWrapper).Envelope.HttpStartStop.InstanceIndex), pubsub.TreeTraverser(_Envelope_HttpStartStop_InstanceId), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_HttpStartStop_InstanceId(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.HttpStartStop.InstanceId == nil {
		return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
			switch idx {
			case 0:
				return 0,
					pubsub.TreeTraverser(func(data interface{}) pubsub.Paths {
						return __ApplicationId
					}), true
			default:
				return 0, nil, false
			}
		})
	}

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 0,
				pubsub.TreeTraverser(func(data interface{}) pubsub.Paths {
					return __ApplicationId
				}), true
		case 1:
			return crc64.Checksum([]byte(*data.(*DataWrapper).Envelope.HttpStartStop.InstanceId), tableECMA),
				pubsub.TreeTraverser(func(data interface{}) pubsub.Paths {
					return __ApplicationId
				}), true
		default:
			return 0, nil, false
		}
	})
}

func __ApplicationId(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
	switch idx {

	case 0:

		if data.(*DataWrapper).Envelope.HttpStartStop.ApplicationId == nil {
			return 0, pubsub.TreeTraverser(done), true
		}

		return 1, pubsub.TreeTraverser(_Envelope_HttpStartStop_ApplicationId_Low), true

	default:
		return 0, nil, false
	}
}

func _Envelope_HttpStartStop_ApplicationId(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.HttpStartStop.ApplicationId == nil {
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
			return 1, pubsub.TreeTraverser(_Envelope_HttpStartStop_ApplicationId_Low), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_HttpStartStop_ApplicationId_Low(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.HttpStartStop.ApplicationId.Low == nil {
		return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
			switch idx {
			case 0:
				return 0, pubsub.TreeTraverser(_Envelope_HttpStartStop_ApplicationId_High), true
			default:
				return 0, nil, false
			}
		})
	}

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 0, pubsub.TreeTraverser(_Envelope_HttpStartStop_ApplicationId_High), true
		case 1:
			return *data.(*DataWrapper).Envelope.HttpStartStop.ApplicationId.Low, pubsub.TreeTraverser(_Envelope_HttpStartStop_ApplicationId_High), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_HttpStartStop_ApplicationId_High(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.HttpStartStop.ApplicationId.High == nil {
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
			return 0, pubsub.TreeTraverser(done), true
		case 1:
			return *data.(*DataWrapper).Envelope.HttpStartStop.ApplicationId.High, pubsub.TreeTraverser(done), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_LogMessage(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.LogMessage == nil {
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
			return 1, pubsub.TreeTraverser(_Envelope_LogMessage_SourceInstance), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_LogMessage_SourceInstance(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.LogMessage.SourceInstance == nil {
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
			return 0, pubsub.TreeTraverser(done), true
		case 1:
			return crc64.Checksum([]byte(*data.(*DataWrapper).Envelope.LogMessage.SourceInstance), tableECMA), pubsub.TreeTraverser(done), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_ValueMetric(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.ValueMetric == nil {
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
			return 1, pubsub.TreeTraverser(_Envelope_ValueMetric_Name), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_ValueMetric_Name(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.ValueMetric.Name == nil {
		return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
			switch idx {
			case 0:
				return 0, pubsub.TreeTraverser(_Envelope_ValueMetric_Value), true
			default:
				return 0, nil, false
			}
		})
	}

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 0, pubsub.TreeTraverser(_Envelope_ValueMetric_Value), true
		case 1:
			return crc64.Checksum([]byte(*data.(*DataWrapper).Envelope.ValueMetric.Name), tableECMA), pubsub.TreeTraverser(_Envelope_ValueMetric_Value), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_ValueMetric_Value(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.ValueMetric.Value == nil {
		return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
			switch idx {
			case 0:
				return 0, pubsub.TreeTraverser(_Envelope_ValueMetric_Unit), true
			default:
				return 0, nil, false
			}
		})
	}

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 0, pubsub.TreeTraverser(_Envelope_ValueMetric_Unit), true
		case 1:
			return uint64(*data.(*DataWrapper).Envelope.ValueMetric.Value), pubsub.TreeTraverser(_Envelope_ValueMetric_Unit), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_ValueMetric_Unit(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.ValueMetric.Unit == nil {
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
			return 0, pubsub.TreeTraverser(done), true
		case 1:
			return crc64.Checksum([]byte(*data.(*DataWrapper).Envelope.ValueMetric.Unit), tableECMA), pubsub.TreeTraverser(done), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_CounterEvent(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.CounterEvent == nil {
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
			return 1, pubsub.TreeTraverser(_Envelope_CounterEvent_Name), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_CounterEvent_Name(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.CounterEvent.Name == nil {
		return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
			switch idx {
			case 0:
				return 0, pubsub.TreeTraverser(_Envelope_CounterEvent_Delta), true
			default:
				return 0, nil, false
			}
		})
	}

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 0, pubsub.TreeTraverser(_Envelope_CounterEvent_Delta), true
		case 1:
			return crc64.Checksum([]byte(*data.(*DataWrapper).Envelope.CounterEvent.Name), tableECMA), pubsub.TreeTraverser(_Envelope_CounterEvent_Delta), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_CounterEvent_Delta(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.CounterEvent.Delta == nil {
		return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
			switch idx {
			case 0:
				return 0, pubsub.TreeTraverser(_Envelope_CounterEvent_Total), true
			default:
				return 0, nil, false
			}
		})
	}

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 0, pubsub.TreeTraverser(_Envelope_CounterEvent_Total), true
		case 1:
			return *data.(*DataWrapper).Envelope.CounterEvent.Delta, pubsub.TreeTraverser(_Envelope_CounterEvent_Total), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_CounterEvent_Total(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.CounterEvent.Total == nil {
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
			return 0, pubsub.TreeTraverser(done), true
		case 1:
			return *data.(*DataWrapper).Envelope.CounterEvent.Total, pubsub.TreeTraverser(done), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_Error(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.Error == nil {
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
			return 1, pubsub.TreeTraverser(_Envelope_Error_Source), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_Error_Source(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.Error.Source == nil {
		return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
			switch idx {
			case 0:
				return 0, pubsub.TreeTraverser(_Envelope_Error_Code), true
			default:
				return 0, nil, false
			}
		})
	}

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 0, pubsub.TreeTraverser(_Envelope_Error_Code), true
		case 1:
			return crc64.Checksum([]byte(*data.(*DataWrapper).Envelope.Error.Source), tableECMA), pubsub.TreeTraverser(_Envelope_Error_Code), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_Error_Code(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.Error.Code == nil {
		return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
			switch idx {
			case 0:
				return 0, pubsub.TreeTraverser(_Envelope_Error_Message), true
			default:
				return 0, nil, false
			}
		})
	}

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 0, pubsub.TreeTraverser(_Envelope_Error_Message), true
		case 1:
			return uint64(*data.(*DataWrapper).Envelope.Error.Code), pubsub.TreeTraverser(_Envelope_Error_Message), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_Error_Message(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.Error.Message == nil {
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
			return 0, pubsub.TreeTraverser(done), true
		case 1:
			return crc64.Checksum([]byte(*data.(*DataWrapper).Envelope.Error.Message), tableECMA), pubsub.TreeTraverser(done), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_ContainerMetric(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.ContainerMetric == nil {
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
			return 1, pubsub.TreeTraverser(_Envelope_ContainerMetric_ApplicationId), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_ContainerMetric_ApplicationId(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.ContainerMetric.ApplicationId == nil {
		return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
			switch idx {
			case 0:
				return 0, pubsub.TreeTraverser(_Envelope_ContainerMetric_InstanceIndex), true
			default:
				return 0, nil, false
			}
		})
	}

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 0, pubsub.TreeTraverser(_Envelope_ContainerMetric_InstanceIndex), true
		case 1:
			return crc64.Checksum([]byte(*data.(*DataWrapper).Envelope.ContainerMetric.ApplicationId), tableECMA), pubsub.TreeTraverser(_Envelope_ContainerMetric_InstanceIndex), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_ContainerMetric_InstanceIndex(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.ContainerMetric.InstanceIndex == nil {
		return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
			switch idx {
			case 0:
				return 0, pubsub.TreeTraverser(_Envelope_ContainerMetric_CpuPercentage), true
			default:
				return 0, nil, false
			}
		})
	}

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 0, pubsub.TreeTraverser(_Envelope_ContainerMetric_CpuPercentage), true
		case 1:
			return uint64(*data.(*DataWrapper).Envelope.ContainerMetric.InstanceIndex), pubsub.TreeTraverser(_Envelope_ContainerMetric_CpuPercentage), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_ContainerMetric_CpuPercentage(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.ContainerMetric.CpuPercentage == nil {
		return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
			switch idx {
			case 0:
				return 0, pubsub.TreeTraverser(_Envelope_ContainerMetric_MemoryBytes), true
			default:
				return 0, nil, false
			}
		})
	}

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 0, pubsub.TreeTraverser(_Envelope_ContainerMetric_MemoryBytes), true
		case 1:
			return uint64(*data.(*DataWrapper).Envelope.ContainerMetric.CpuPercentage), pubsub.TreeTraverser(_Envelope_ContainerMetric_MemoryBytes), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_ContainerMetric_MemoryBytes(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.ContainerMetric.MemoryBytes == nil {
		return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
			switch idx {
			case 0:
				return 0, pubsub.TreeTraverser(_Envelope_ContainerMetric_DiskBytes), true
			default:
				return 0, nil, false
			}
		})
	}

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 0, pubsub.TreeTraverser(_Envelope_ContainerMetric_DiskBytes), true
		case 1:
			return *data.(*DataWrapper).Envelope.ContainerMetric.MemoryBytes, pubsub.TreeTraverser(_Envelope_ContainerMetric_DiskBytes), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_ContainerMetric_DiskBytes(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.ContainerMetric.DiskBytes == nil {
		return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
			switch idx {
			case 0:
				return 0, pubsub.TreeTraverser(_Envelope_ContainerMetric_MemoryBytesQuota), true
			default:
				return 0, nil, false
			}
		})
	}

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 0, pubsub.TreeTraverser(_Envelope_ContainerMetric_MemoryBytesQuota), true
		case 1:
			return *data.(*DataWrapper).Envelope.ContainerMetric.DiskBytes, pubsub.TreeTraverser(_Envelope_ContainerMetric_MemoryBytesQuota), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_ContainerMetric_MemoryBytesQuota(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.ContainerMetric.MemoryBytesQuota == nil {
		return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
			switch idx {
			case 0:
				return 0, pubsub.TreeTraverser(_Envelope_ContainerMetric_DiskBytesQuota), true
			default:
				return 0, nil, false
			}
		})
	}

	return pubsub.Paths(func(idx int, data interface{}) (path uint64, nextTraverser pubsub.TreeTraverser, ok bool) {
		switch idx {
		case 0:
			return 0, pubsub.TreeTraverser(_Envelope_ContainerMetric_DiskBytesQuota), true
		case 1:
			return *data.(*DataWrapper).Envelope.ContainerMetric.MemoryBytesQuota, pubsub.TreeTraverser(_Envelope_ContainerMetric_DiskBytesQuota), true
		default:
			return 0, nil, false
		}
	})
}

func _Envelope_ContainerMetric_DiskBytesQuota(data interface{}) pubsub.Paths {

	if data.(*DataWrapper).Envelope.ContainerMetric.DiskBytesQuota == nil {
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
			return 0, pubsub.TreeTraverser(done), true
		case 1:
			return *data.(*DataWrapper).Envelope.ContainerMetric.DiskBytesQuota, pubsub.TreeTraverser(done), true
		default:
			return 0, nil, false
		}
	})
}

type DataWrapperFilter struct {
	AppID    *string
	Envelope *eventsEnvelopeFilter
}

type eventsEnvelopeFilter struct {
	Ip              *string
	HttpStartStop   *HttpStartStopFilter
	LogMessage      *LogMessageFilter
	ValueMetric     *ValueMetricFilter
	CounterEvent    *CounterEventFilter
	Error           *ErrorFilter
	ContainerMetric *ContainerMetricFilter
}

type HttpStartStopFilter struct {
	RemoteAddress *string
	UserAgent     *string
	StatusCode    *int32
	ContentLength *int64
	InstanceIndex *int32
	InstanceId    *string
	ApplicationId *UUIDFilter
}

type UUIDFilter struct {
	Low  *uint64
	High *uint64
}

type LogMessageFilter struct {
	SourceInstance *string
}

type ValueMetricFilter struct {
	Name  *string
	Value *float64
	Unit  *string
}

type CounterEventFilter struct {
	Name  *string
	Delta *uint64
	Total *uint64
}

type ErrorFilter struct {
	Source  *string
	Code    *int32
	Message *string
}

type ContainerMetricFilter struct {
	ApplicationId    *string
	InstanceIndex    *int32
	CpuPercentage    *float64
	MemoryBytes      *uint64
	DiskBytes        *uint64
	MemoryBytesQuota *uint64
	DiskBytesQuota   *uint64
}

func envelopeTraverserCreatePath(f *DataWrapperFilter) []uint64 {
	if f == nil {
		return nil
	}
	var path []uint64

	var count int
	if f.Envelope != nil {
		count++
	}

	if count > 1 {
		panic("Only one field can be set")
	}

	if f.AppID != nil {
		path = append(path, crc64.Checksum([]byte(*f.AppID), tableECMA))
	} else {
		path = append(path, 0)
	}

	path = append(path, createPath_Envelope(f.Envelope)...)

	for i := len(path) - 1; i >= 1; i-- {
		if path[i] != 0 {
			break
		}
		path = path[:i]
	}

	return path
}

func createPath_Envelope(f *eventsEnvelopeFilter) []uint64 {
	if f == nil {
		return nil
	}
	var path []uint64

	path = append(path, 1)

	var count int
	if f.HttpStartStop != nil {
		count++
	}

	if f.LogMessage != nil {
		count++
	}

	if f.ValueMetric != nil {
		count++
	}

	if f.CounterEvent != nil {
		count++
	}

	if f.Error != nil {
		count++
	}

	if f.ContainerMetric != nil {
		count++
	}

	if count > 1 {
		panic("Only one field can be set")
	}

	if f.Ip != nil {
		path = append(path, crc64.Checksum([]byte(*f.Ip), tableECMA))
	} else {
		path = append(path, 0)
	}

	path = append(path, createPath_HttpStartStop(f.HttpStartStop)...)

	path = append(path, createPath_LogMessage(f.LogMessage)...)

	path = append(path, createPath_ValueMetric(f.ValueMetric)...)

	path = append(path, createPath_CounterEvent(f.CounterEvent)...)

	path = append(path, createPath_Error(f.Error)...)

	path = append(path, createPath_ContainerMetric(f.ContainerMetric)...)

	return path
}

func createPath_HttpStartStop(f *HttpStartStopFilter) []uint64 {
	if f == nil {
		return nil
	}
	var path []uint64

	path = append(path, 1)

	var count int
	if f.ApplicationId != nil {
		count++
	}

	if count > 1 {
		panic("Only one field can be set")
	}

	if f.RemoteAddress != nil {
		path = append(path, crc64.Checksum([]byte(*f.RemoteAddress), tableECMA))
	} else {
		path = append(path, 0)
	}

	if f.UserAgent != nil {
		path = append(path, crc64.Checksum([]byte(*f.UserAgent), tableECMA))
	} else {
		path = append(path, 0)
	}

	if f.StatusCode != nil {
		path = append(path, uint64(*f.StatusCode))
	} else {
		path = append(path, 0)
	}

	if f.ContentLength != nil {
		path = append(path, uint64(*f.ContentLength))
	} else {
		path = append(path, 0)
	}

	if f.InstanceIndex != nil {
		path = append(path, uint64(*f.InstanceIndex))
	} else {
		path = append(path, 0)
	}

	if f.InstanceId != nil {
		path = append(path, crc64.Checksum([]byte(*f.InstanceId), tableECMA))
	} else {
		path = append(path, 0)
	}

	path = append(path, createPath_ApplicationId(f.ApplicationId)...)

	return path
}

func createPath_ApplicationId(f *UUIDFilter) []uint64 {
	if f == nil {
		return nil
	}
	var path []uint64

	path = append(path, 1)

	var count int
	if count > 1 {
		panic("Only one field can be set")
	}

	if f.Low != nil {
		path = append(path, *f.Low)
	} else {
		path = append(path, 0)
	}

	if f.High != nil {
		path = append(path, *f.High)
	} else {
		path = append(path, 0)
	}

	return path
}

func createPath_LogMessage(f *LogMessageFilter) []uint64 {
	if f == nil {
		return nil
	}
	var path []uint64

	path = append(path, 2)

	var count int
	if count > 1 {
		panic("Only one field can be set")
	}

	if f.SourceInstance != nil {
		path = append(path, crc64.Checksum([]byte(*f.SourceInstance), tableECMA))
	} else {
		path = append(path, 0)
	}

	return path
}

func createPath_ValueMetric(f *ValueMetricFilter) []uint64 {
	if f == nil {
		return nil
	}
	var path []uint64

	path = append(path, 3)

	var count int
	if count > 1 {
		panic("Only one field can be set")
	}

	if f.Name != nil {
		path = append(path, crc64.Checksum([]byte(*f.Name), tableECMA))
	} else {
		path = append(path, 0)
	}

	if f.Value != nil {
		path = append(path, uint64(*f.Value))
	} else {
		path = append(path, 0)
	}

	if f.Unit != nil {
		path = append(path, crc64.Checksum([]byte(*f.Unit), tableECMA))
	} else {
		path = append(path, 0)
	}

	return path
}

func createPath_CounterEvent(f *CounterEventFilter) []uint64 {
	if f == nil {
		return nil
	}
	var path []uint64

	path = append(path, 4)

	var count int
	if count > 1 {
		panic("Only one field can be set")
	}

	if f.Name != nil {
		path = append(path, crc64.Checksum([]byte(*f.Name), tableECMA))
	} else {
		path = append(path, 0)
	}

	if f.Delta != nil {
		path = append(path, *f.Delta)
	} else {
		path = append(path, 0)
	}

	if f.Total != nil {
		path = append(path, *f.Total)
	} else {
		path = append(path, 0)
	}

	return path
}

func createPath_Error(f *ErrorFilter) []uint64 {
	if f == nil {
		return nil
	}
	var path []uint64

	path = append(path, 5)

	var count int
	if count > 1 {
		panic("Only one field can be set")
	}

	if f.Source != nil {
		path = append(path, crc64.Checksum([]byte(*f.Source), tableECMA))
	} else {
		path = append(path, 0)
	}

	if f.Code != nil {
		path = append(path, uint64(*f.Code))
	} else {
		path = append(path, 0)
	}

	if f.Message != nil {
		path = append(path, crc64.Checksum([]byte(*f.Message), tableECMA))
	} else {
		path = append(path, 0)
	}

	return path
}

func createPath_ContainerMetric(f *ContainerMetricFilter) []uint64 {
	if f == nil {
		return nil
	}
	var path []uint64

	path = append(path, 6)

	var count int
	if count > 1 {
		panic("Only one field can be set")
	}

	if f.ApplicationId != nil {
		path = append(path, crc64.Checksum([]byte(*f.ApplicationId), tableECMA))
	} else {
		path = append(path, 0)
	}

	if f.InstanceIndex != nil {
		path = append(path, uint64(*f.InstanceIndex))
	} else {
		path = append(path, 0)
	}

	if f.CpuPercentage != nil {
		path = append(path, uint64(*f.CpuPercentage))
	} else {
		path = append(path, 0)
	}

	if f.MemoryBytes != nil {
		path = append(path, *f.MemoryBytes)
	} else {
		path = append(path, 0)
	}

	if f.DiskBytes != nil {
		path = append(path, *f.DiskBytes)
	} else {
		path = append(path, 0)
	}

	if f.MemoryBytesQuota != nil {
		path = append(path, *f.MemoryBytesQuota)
	} else {
		path = append(path, 0)
	}

	if f.DiskBytesQuota != nil {
		path = append(path, *f.DiskBytesQuota)
	} else {
		path = append(path, 0)
	}

	return path
}
