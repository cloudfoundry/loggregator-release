package profiler

import (
	"fmt"
	"net/http"
)

// Starter is something that can be started
type Starter interface {
	Start()
}

type errorLogger interface {
	Errorf(msg string, values ...interface{})
}

// New creates a profiler server
func New(port uint32, logger errorLogger) Starter {
	return &server{port, logger}
}

type server struct {
	port   uint32
	logger errorLogger
}

// Start initializes a profiler server on a port
func (s *server) Start() {
	err := http.ListenAndServe(fmt.Sprintf("localhost:%d", s.port), nil)
	if err != nil {
		s.logger.Errorf("Error starting pprof server: %s", err.Error())
	}
}
