package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

var port = flag.Int("port", 1234, "port to listen on")
var path = flag.String("path", "/syslog/", "path to listen on")
var certFile = flag.String("cert", "", "certificate file")
var keyFile = flag.String("key", "", "key file")

func main() {
	flag.Parse()
	log.Fatal(startServer(*port, *path, *certFile, *keyFile))
}

func handler(_ http.ResponseWriter, r *http.Request) {
	bytes := make([]byte, 1024)
	byteCount, _ := r.Body.Read(bytes)
	r.Body.Close()
	fmt.Printf("%s\n", bytes[:byteCount])
}

func startServer(port int, path string, cert string, key string) error {
	mux := http.NewServeMux()
	mux.HandleFunc(path, handler)
	srv := http.Server{
		Handler: mux,
		Addr:    fmt.Sprintf(":%d", port),
	}

	return srv.ListenAndServeTLS(cert, key)
}
