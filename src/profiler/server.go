package profiler

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
)

// Starter is something that can be started
type Starter interface {
	Start()
}

// New creates a profiler server
func New(port uint32) Starter {
	return &server{port}
}

type server struct {
	port uint32
}

// Start initializes a profiler server on a port
func (s *server) Start() {
	err := http.ListenAndServe(fmt.Sprintf("localhost:%d", s.port), nil)
	if err != nil {
		log.Printf("Error starting pprof server: %s", err)
	}
}
