package trafficcontroller

import (
	"github.com/cloudfoundry/gosteno"
	"mime/multipart"
	"net/http"
	"sync"
	"trafficcontroller/hasher"
	"trafficcontroller/listener"
)

var NewWebsocketListener = func() listener.Listener {
	return listener.NewWebsocket()
}

type HttpHandler struct {
	hashers []*hasher.Hasher
	logger  *gosteno.Logger
	sync.WaitGroup
	sync.Mutex
}

func NewHttpHandler(hashers []*hasher.Hasher, logger *gosteno.Logger) *HttpHandler {
	return &HttpHandler{hashers: hashers, logger: logger}
}

func (h *HttpHandler) ServeHttp(rw http.ResponseWriter, r *http.Request) {
	mp := multipart.NewWriter(rw)

	defer func() {
		h.Lock()
		defer h.Unlock()
		mp.Close()
	}()

	func() {
		h.Lock()
		defer h.Unlock()
		mp.SetBoundary("loggregator-message")
	}()

	rw.Header().Set("Content-Type", `multipart/x-protobuf; boundary="`+mp.Boundary()+`"`)

	if len(h.hashers) == 0 {
		return
	}

	r.ParseForm()
	appId := r.Form.Get("app")

	messages := make(chan []byte)
	h.Add(len(h.hashers))
	for _, hasher := range h.hashers {
		loggregatorAddress := hasher.GetLoggregatorServerForAppId(appId)
		go h.proxyConnection(messages, "ws://"+loggregatorAddress+"/dump/?app="+appId)
	}

	go h.writeMultiPartResponse(mp, messages)

	h.Wait()
	close(messages)
}

func (h *HttpHandler) proxyConnection(outgoing chan<- []byte, serverAddress string) {
	defer h.Done()

	h.logger.Debugf("HttpHandler: Grabbing messages from " + serverAddress)

	l := NewWebsocketListener()
	incoming, _ := l.Start(serverAddress)

	if len(incoming) == 0 {
		h.logger.Debugf("HttpHandler: no messages from " + serverAddress)
		return
	}

	for message := range incoming {
		outgoing <- message
	}
	h.logger.Debugf("HttpHandler: processed all messages from " + serverAddress)
}

func (h *HttpHandler) writeMultiPartResponse(mp *multipart.Writer, messages <-chan []byte) {
	for message := range messages {
		func() {
			h.Lock()
			defer h.Unlock()
			partWriter, _ := mp.CreatePart(make(map[string][]string))
			partWriter.Write(message)
		}()
	}
}
