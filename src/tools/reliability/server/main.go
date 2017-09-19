package main

import (
	"log"
	"net/http"
	"os"
	"time"
	"tools/reliability/server/internal/api"
)

func main() {
	port := os.Getenv("PORT")

	workerHandler := api.NewWorkerHandler()

	http.Handle("/tests", api.NewCreateTestHandler(workerHandler, 5*time.Second))
	http.Handle("/workers", workerHandler)

	addr := ":" + port
	log.Printf("server started on %s", addr)
	log.Println(http.ListenAndServe(addr, nil))
}
