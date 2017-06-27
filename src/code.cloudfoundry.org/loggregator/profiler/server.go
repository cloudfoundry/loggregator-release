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
	Addr() net.Addr
}

// New creates a profiler server
func New(port uint32) Starter {
	addr := fmt.Sprintf("localhost:%d", port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Panicf("Error creating pprof listener: %s", err)
	}

	return &server{lis}
}

type server struct {
	listener net.Listener
}

// Start initializes a profiler server on a port
func (s *server) Start() {
	log.Printf("Starting pprof server on: %s", s.listener.Addr().String())
	err := http.Serve(s.listener, nil)
	if err != nil {
		log.Panicf("Error starting pprof server: %s", err)
	}
}

func (s *server) Addr() net.Addr {
	return s.listener.Addr()
}
