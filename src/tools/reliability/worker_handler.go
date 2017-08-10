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

type WorkerHandler struct {
	mu    sync.RWMutex
	conns []*websocket.Conn
}

func NewWorkerHandler() *WorkerHandler {
	return &WorkerHandler{}
}

func (s *WorkerHandler) Run(t *Test) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, c := range s.conns {
		err := c.WriteJSON(&t)
		if err != nil {
			log.Printf("Failed emit test: %s", err)
		}
	}
}

func (s *WorkerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("failed to upgrade request to WS: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer conn.Close()

	s.mu.Lock()
	s.conns = append(s.conns, conn)
	s.mu.Unlock()

	log.Println("worker has connected")

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Printf("read failed: %s", err)
			break
		}
	}

	s.mu.Lock()
	idx := -1
	for i, c := range s.conns {
		if conn == c {
			idx = i
			break
		}
	}

	if idx > -1 {
		s.conns = append(s.conns[:idx], s.conns[idx+1:]...)
	}
	s.mu.Unlock()
	log.Println("worker has been removed")
}
