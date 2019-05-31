package auth

import (
	"log"
	"net/http"
)

func Access(accessLogger AccessLogger, host string, port uint32) func(http.Handler) *AccessHandler {
	return func(handler http.Handler) *AccessHandler {
		return &AccessHandler{
			handler:      handler,
			accessLogger: accessLogger,
			host:         host,
			port:         port,
		}
	}
}

type AccessLogger interface {
	LogAccess(req *http.Request, host string, port uint32) error
}

type AccessHandler struct {
	handler      http.Handler
	accessLogger AccessLogger
	host         string
	port         uint32
}

func (h *AccessHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if err := h.accessLogger.LogAccess(req, h.host, h.port); err != nil {
		log.Printf("access handler : %s", err)
	}

	h.handler.ServeHTTP(rw, req)
}
