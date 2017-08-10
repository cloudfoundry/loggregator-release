package main

import (
	"log"
	"net/http"
	"os"

	"tools/reliability"
)

func main() {
	port := os.Getenv("PORT")

	workerHandler := reliability.NewWorkerHandler()

	http.Handle("/tests", reliability.NewCreateTestHandler(workerHandler))
	http.Handle("/workers", workerHandler)

	addr := ":" + port
	log.Printf("server started on %s", addr)
	log.Println(http.ListenAndServe(addr, nil))
}
