package reliability

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// WorkerHandler is a websocket handler that waits for Worker connections.
// It keeps track of each connection, so that when a test is started (via
// Run()), it can tell each connection about the test.
type WorkerHandler struct {
	mu    sync.RWMutex
	conns map[*websocket.Conn]struct{}
}

// NewWorkerHandler builds a new WorkerHandler.
func NewWorkerHandler() *WorkerHandler {
	return &WorkerHandler{
		conns: make(map[*websocket.Conn]struct{}),
	}
}

// Run writes the test information to each websocket connection.
func (s *WorkerHandler) Run(t *Test) {
	var conns []*websocket.Conn
	s.mu.RLock()
	for conn := range s.conns {
		conns = append(conns, conn)
	}
	s.mu.RUnlock()

	// Ensure each worker only writes the number of logs to stdout that will
	// equate to the desired count.
	t.WriteCycles = t.Cycles / uint64(len(conns))
	remainder := t.Cycles % uint64(len(conns))

	for i, c := range conns {
		if i == len(conns)-1 {
			t.WriteCycles += remainder
		}
		err := c.WriteJSON(&t)
		if err != nil {
			log.Printf("Failed emit test: %s", err)
		}
	}
}

// ServeHTTP implements http.Handler. It only accepts websocket connections.
func (s *WorkerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("failed to upgrade request to WS: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer conn.Close()

	s.mu.Lock()
	s.conns[conn] = struct{}{}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		delete(s.conns, conn)
		log.Println("worker has been removed")
		conn.Close()
	}()

	log.Println("worker has connected")

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Printf("read failed: %s", err)
			break
		}
	}
}
