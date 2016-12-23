package doppler_endpoint

import (
	"fmt"
	"net/http"
	"time"

	"github.com/cloudfoundry/loggregatorlib/server/handlers"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

var WebsocketKeepAliveDuration = 30 * time.Second

const HttpRequestTimeout = 5 * time.Second

type DopplerEndpoint struct {
	Endpoint  string
	StreamId  string
	Reconnect bool
	Timeout   time.Duration
	HProvider HandlerProvider
}

func NewDopplerEndpoint(endpoint string,
	streamId string,
	reconnect bool) DopplerEndpoint {

	var hProvider HandlerProvider
	var timeout time.Duration
	if endpoint == "recentlogs" {
		timeout = HttpRequestTimeout
		hProvider = HttpHandlerProvider
	} else if endpoint == "containermetrics" {
		timeout = HttpRequestTimeout
		hProvider = ContainerMetricHandlerProvider
	} else {
		hProvider = WebsocketHandlerProvider
	}

	return DopplerEndpoint{
		Endpoint:  endpoint,
		StreamId:  streamId,
		Reconnect: reconnect,
		Timeout:   timeout,
		HProvider: hProvider,
	}
}

type HandlerProvider func(<-chan []byte) http.Handler

func HttpHandlerProvider(messages <-chan []byte) http.Handler {
	return handlers.NewHttpHandler(messages)
}

func WebsocketHandlerProvider(messages <-chan []byte) http.Handler {
	return handlers.NewWebsocketHandler(messages, WebsocketKeepAliveDuration)
}

func ContainerMetricHandlerProvider(messages <-chan []byte) http.Handler {
	outputChan := DeDupe(messages)
	return handlers.NewHttpHandler(outputChan)
}

func (endpoint *DopplerEndpoint) GetPath() string {
	if endpoint.Endpoint == "firehose" {
		return "/firehose/" + endpoint.StreamId
	} else {
		return fmt.Sprintf("/apps/%s/%s", endpoint.StreamId, endpoint.Endpoint)
	}
}

func DeDupe(input <-chan []byte) <-chan []byte {
	messages := make(map[int32]*events.Envelope)
	for message := range input {
		var envelope events.Envelope
		proto.Unmarshal(message, &envelope)
		cm := envelope.GetContainerMetric()

		oldEnvelope, ok := messages[cm.GetInstanceIndex()]
		if !ok || oldEnvelope.GetTimestamp() < envelope.GetTimestamp() {
			messages[cm.GetInstanceIndex()] = &envelope
		}
	}

	output := make(chan []byte, len(messages))

	for _, envelope := range messages {
		bytes, _ := proto.Marshal(envelope)
		output <- bytes
	}
	close(output)
	return output
}
