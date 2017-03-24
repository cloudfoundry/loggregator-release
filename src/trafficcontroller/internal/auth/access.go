package auth

import (
	"log"
	"net/http"
)

//go:generate hel --type HttpHandler --output mock_http_writer_test.go

type HttpHandler interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
}

//go:generate hel --type AccessLogger --output mock_access_logger_test.go

type AccessLogger interface {
	LogAccess(req *http.Request, host string, port uint32) error
}

type AccessHandler struct {
	handler      HttpHandler
	accessLogger AccessLogger
	host         string
	port         uint32
}

func Access(accessLogger AccessLogger, host string, port uint32) func(HttpHandler) *AccessHandler {
	return func(handler HttpHandler) *AccessHandler {
		return &AccessHandler{
			handler:      handler,
			accessLogger: accessLogger,
			host:         host,
			port:         port,
		}
	}
}

func (h *AccessHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if err := h.accessLogger.LogAccess(req, h.host, h.port); err != nil {
		log.Printf("access handler : %s", err)
	}

	h.handler.ServeHTTP(rw, req)
}
