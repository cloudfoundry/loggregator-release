package v1

import (
	"code.cloudfoundry.org/loggregator/plumbing"

	"github.com/apoydence/pubsub"
	"github.com/apoydence/pubsub/pubsub-gen/setters"
	"github.com/cloudfoundry/sonde-go/events"
)

//go:generate pubsub-gen --pointer --output=$GOPATH/src/code.cloudfoundry.org/loggregator/doppler/internal/grpcmanager/v1/envelope_traverser.gen.go -package=v1 --sub-structs={"events.Envelope":"github.com/cloudfoundry/sonde-go/events"} --struct-name=code.cloudfoundry.org/loggregator/doppler/internal/grpcmanager/v1.DataWrapper --traverser=envelopeTraverser --blacklist-fields=*.XXX_unrecognized,Envelope.EventType,HttpStartStop.PeerType,HttpStartStop.Method,LogMessage.MessageType,Envelope.Timestamp,Envelope.Origin,Envelope.Job,Envelope.Deployment,Envelope.Index,HttpStartStop.StartTimestamp,HttpStartStop.StopTimestamp,HttpStartStop.RequestId,HttpStartStop.Uri,Envelope.RemoteAddress,Envelope.UserAgent,Envelope.StatusCode,Envelope.ContentLength,Envelope.ApplicationId,Envelope.InstanceIndex,Envelope.InstanceId,Envelope.Forwarded,LogMessage.Timestamp,LogMessage.AppId,LogMessage.SourceType

type Router struct {
	ps *pubsub.PubSub
}

func NewRouter() *Router {
	return &Router{
		ps: pubsub.New(),
	}
}

type DataWrapper struct {
	Envelope   *events.Envelope
	AppID      string
	Marshalled []byte
}

func (r *Router) Register(req *plumbing.SubscriptionRequest, dataSetter DataSetter) (cleanup func()) {
	subscription := r.buildSubscription(dataSetter, req)
	var cleanups []func()
	for _, path := range r.convertFilter(req) {
		c := r.ps.Subscribe(subscription,
			pubsub.WithPath(path),
			pubsub.WithShardID(req.ShardID),
		)
		cleanups = append(cleanups, c)
	}

	return func() {
		for _, c := range cleanups {
			c()
		}
	}
}

func (r *Router) SendTo(appID string, envelope *events.Envelope) {
	data := r.marshal(envelope)
	if data == nil {
		return
	}

	wrapper := &DataWrapper{
		AppID:      appID,
		Marshalled: data,
		Envelope:   envelope,
	}

	r.ps.Publish(wrapper, envelopeTraverserTraverse)
}

func (r *Router) marshal(envelope *events.Envelope) []byte {
	data, err := envelope.Marshal()
	if err != nil {
		return nil
	}

	return data
}

func (r *Router) buildSubscription(
	dataSetter DataSetter,
	req *plumbing.SubscriptionRequest,
) pubsub.Subscription {

	return pubsub.Subscription(func(data interface{}) {
		dw := data.(*DataWrapper)
		if !r.validateData(dw, req) {
			return
		}

		dataSetter(dw.Marshalled)
	})
}

// validateData ensures there weren't any collisions when hasing the values
// and the pubsub sent us data we don't want.
func (r *Router) validateData(dw *DataWrapper, req *plumbing.SubscriptionRequest) bool {
	if req.Filter == nil {
		return true
	}

	if req.Filter.AppID != "" && req.Filter.AppID != dw.AppID {
		return false
	}

	switch req.Filter.Message.(type) {
	case *plumbing.Filter_Log:
		return dw.Envelope.LogMessage != nil
	case *plumbing.Filter_Metric:
		return dw.Envelope.HttpStartStop != nil ||
			dw.Envelope.ValueMetric != nil ||
			dw.Envelope.CounterEvent != nil ||
			dw.Envelope.ContainerMetric != nil
	default:
		return true
	}
}

func (r *Router) convertFilter(req *plumbing.SubscriptionRequest) [][]uint64 {
	if req.GetFilter() == nil {
		return [][]uint64{nil}
	}

	if req.Filter.AppID != "" {
		return r.createAppIDPaths(req)
	}

	logPath := &eventsEnvelopeFilter{
		LogMessage: &LogMessageFilter{},
	}

	metricPath := []*eventsEnvelopeFilter{
		{HttpStartStop: &HttpStartStopFilter{}},
		{ValueMetric: &ValueMetricFilter{}},
		{CounterEvent: &CounterEventFilter{}},
		{ContainerMetric: &ContainerMetricFilter{}},
	}

	switch req.Filter.Message.(type) {
	case *plumbing.Filter_Log:
		return r.createPaths(nil, logPath)
	case *plumbing.Filter_Metric:
		return r.createPaths(nil, metricPath...)
	default:
		return r.createPaths(nil, append(metricPath, logPath)...)
	}
}

func (r *Router) createAppIDPaths(req *plumbing.SubscriptionRequest) [][]uint64 {
	logPath := &eventsEnvelopeFilter{
		LogMessage: &LogMessageFilter{},
	}

	metricPath := []*eventsEnvelopeFilter{
		{HttpStartStop: &HttpStartStopFilter{}},
		{ContainerMetric: &ContainerMetricFilter{}},
	}

	switch req.Filter.Message.(type) {
	case *plumbing.Filter_Log:
		return r.createPaths(setters.String(req.Filter.AppID), logPath)
	case *plumbing.Filter_Metric:
		return r.createPaths(setters.String(req.Filter.AppID), metricPath...)
	default:
		return [][]uint64{envelopeTraverserCreatePath(&DataWrapperFilter{AppID: setters.String(req.Filter.AppID)})}
	}
}

func (r *Router) createPaths(appID *string, f ...*eventsEnvelopeFilter) [][]uint64 {
	var paths [][]uint64
	for _, ff := range f {
		paths = append(paths, envelopeTraverserCreatePath(&DataWrapperFilter{AppID: appID, Envelope: ff}))
	}
	return paths
}
