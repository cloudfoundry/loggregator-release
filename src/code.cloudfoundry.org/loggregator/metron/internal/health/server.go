package health

import (
	"fmt"
	"log"
	"net/http"
)

type Server struct {
	port     uint
	registry *Registry
}

func NewServer(port uint, r *Registry) *Server {
	return &Server{port: port, registry: r}
}

func (s *Server) Run() {
	http.Handle("/health", NewHandler(s.registry))

	log.Print(http.ListenAndServe(fmt.Sprintf(":%d", s.port), nil))
}
