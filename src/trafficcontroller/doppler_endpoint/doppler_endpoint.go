package doppler_endpoint

import (
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/server/handlers"
	"net/http"
	"time"
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

func NewDopplerEndpoint(
	endpoint string,
	streamId string,
	reconnect bool,
) DopplerEndpoint {

	var hProvider HandlerProvider
	var timeout time.Duration
	if endpoint == "recentlogs" {
		hProvider = HttpHandlerProvider
		timeout = HttpRequestTimeout
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

type HandlerProvider func(<-chan []byte, *gosteno.Logger) http.Handler

func HttpHandlerProvider(messages <-chan []byte, logger *gosteno.Logger) http.Handler {
	return handlers.NewHttpHandler(messages, logger)
}

func WebsocketHandlerProvider(messages <-chan []byte, logger *gosteno.Logger) http.Handler {
	return handlers.NewWebsocketHandler(messages, WebsocketKeepAliveDuration, logger)
}

func (endpoint *DopplerEndpoint) GetPath() string {
	if endpoint.Endpoint == "firehose" {
		return "/firehose/" + endpoint.StreamId
	} else {
		return fmt.Sprintf("/apps/%s/%s", endpoint.StreamId, endpoint.Endpoint)
	}
}
