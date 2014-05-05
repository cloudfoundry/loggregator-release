package handlers

import (
	"github.com/cloudfoundry/gosteno"
	"mime/multipart"
	"net/http"
)

type httpHandler struct {
	messages <-chan []byte
	logger   *gosteno.Logger
}

func NewHttpHandler(m <-chan []byte, logger *gosteno.Logger) *httpHandler {
	return &httpHandler{messages: m, logger: logger}
}

func (h *httpHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	mp := multipart.NewWriter(rw)
	defer mp.Close()

	rw.Header().Set("Content-Type", `multipart/x-protobuf; boundary=`+mp.Boundary())

	for message := range h.messages {
		partWriter, _ := mp.CreatePart(nil)
		partWriter.Write(message)
	}
}
