package health

func New(port uint) (*Server, *Registry) {
	registry := NewRegistry()
	server := NewServer(port, registry)

	return server, registry
}
