package doppler_endpoint

import (
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/server/handlers"
	"net/http"
	"time"
)

var WebsocketKeepAliveDuration = 30 * time.Second

type DopplerEndpoint struct {
	Endpoint  string
	StreamId  string
	Reconnect bool
	HProvider HandlerProvider
}

func NewDopplerEndpoint(
	endpoint string,
	streamId string,
	reconnect bool,
) DopplerEndpoint {

	var hProvider HandlerProvider
	if endpoint == "recentlogs" {
		hProvider = HttpHandlerProvider
	} else {
		hProvider = WebsocketHandlerProvider

	}

	return DopplerEndpoint{
		Endpoint:  endpoint,
		StreamId:  streamId,
		Reconnect: reconnect,
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
		return "/" + endpoint.Endpoint
	} else {
		return fmt.Sprintf("/apps/%s/%s", endpoint.StreamId, endpoint.Endpoint)
	}
}
