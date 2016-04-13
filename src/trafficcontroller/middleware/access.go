package middleware

import (
	"net/http"

	"github.com/cloudfoundry/gosteno"
)

//go:generate hel --type HttpHandler --output mock_http_writer_test.go

type HttpHandler interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
}

//go:generate hel --type AccessLogger --output mock_access_logger_test.go

type AccessLogger interface {
	LogAccess(req *http.Request) error
}

type AccessHandler struct {
	handler      HttpHandler
	accessLogger AccessLogger
	logger       *gosteno.Logger
}

func Access(accessLogger AccessLogger, logger *gosteno.Logger) func(HttpHandler) *AccessHandler {
	return func(handler HttpHandler) *AccessHandler {
		return &AccessHandler{
			handler:      handler,
			accessLogger: accessLogger,
			logger:       logger,
		}
	}
}

func (h *AccessHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if err := h.accessLogger.LogAccess(req); err != nil {
		h.logger.Errorf("access handler : %s", err)
	}

	h.handler.ServeHTTP(rw, req)
}
