package proxy

import (
	"mime/multipart"
	"net/http"
)

type HttpHandler struct {
	Messages <-chan []byte
}

func NewHttpHandler(m <-chan []byte) *HttpHandler {
	return &HttpHandler{Messages: m}
}

func (h *HttpHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	mp := multipart.NewWriter(rw)
	defer mp.Close()

	rw.Header().Set("Content-Type", `multipart/x-protobuf; boundary=`+mp.Boundary())

	for message := range h.Messages {
		partWriter, err := mp.CreatePart(nil)
		if err != nil {
			return
		}

		partWriter.Write(message)
	}
}
