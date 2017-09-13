package profiler

import (
	"fmt"
	"log"
	"net"
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
	addr := fmt.Sprintf("localhost:%d", s.port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Panicf("Error creating pprof listener: %s", err)
	}

	log.Printf("pprof bound to: %s", lis.Addr())
	err = http.Serve(lis, nil)
	if err != nil {
		log.Panicf("Error starting pprof server: %s", err)
	}
}
