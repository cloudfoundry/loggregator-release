package health

import (
	"fmt"
	"log"
	"net"
	"net/http"
)

type Server struct {
	listener net.Listener
	registry *Registry
}

func NewServer(port uint, r *Registry) *Server {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Panicf("unable to listen for health: %s", err)
	}
	return &Server{
		listener: listener,
		registry: r,
	}
}

func (s *Server) Run() {
	http.Handle("/health", NewHandler(s.registry))
	log.Print(http.Serve(s.listener, nil))
}

func (s *Server) Addr() net.Addr {
	return s.listener.Addr()
}
