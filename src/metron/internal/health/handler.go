package health

import (
	"encoding/json"
	"log"
	"net/http"
)

func NewHandler(r *Registry) http.Handler {
	return &handler{registry: r}
}

type handler struct {
	registry *Registry
}

func (h *handler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	err := json.NewEncoder(rw).Encode(h.registry.State())
	if err != nil {
		log.Println("Failed to encode health response: %s", err)
		rw.WriteHeader(http.StatusInternalServerError)
	}
}
