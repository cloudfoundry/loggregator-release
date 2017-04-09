package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

func main() {
	port := flag.Int("port", 1234, "port to listen on")
	path := flag.String("path", "/syslog/", "path to listen on")
	certPath := flag.String("cert", "", "certificate file")
	keyPath := flag.String("key", "", "key file")
	flag.Parse()

	s := NewServer(*port, *path, *certPath, *keyPath)
	s.Start()
}

func NewServer(port int, path, certPath, keyPath string) *server {
	mux := http.NewServeMux()
	mux.HandleFunc(path, echoHandler)
	srv := http.Server{
		Handler: mux,
		Addr:    fmt.Sprintf(":%d", port),
	}

	return &server{
		certPath: certPath,
		keyPath:  keyPath,
		srv:      srv,
	}
}

type server struct {
	certPath string
	keyPath  string
	srv      http.Server
}

func (s *server) Start() {
	log.Fatal(s.srv.ListenAndServeTLS(s.certPath, s.keyPath))
}

func echoHandler(_ http.ResponseWriter, r *http.Request) {
	bytes := make([]byte, 1024)
	byteCount, _ := r.Body.Read(bytes)
	r.Body.Close()
	fmt.Printf("%s\n", bytes[:byteCount])
}
