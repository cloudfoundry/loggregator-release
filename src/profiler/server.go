package profiler

import (
	"fmt"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
)

// New creates a profiler server
func New(port uint32) *Server {
	return &Server{port}
}

// Server is an http server with the pprof listener enabled.
type Server struct {
	port uint32
}

// Start initializes a profiler server on a port
func (s *Server) Start() {
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
