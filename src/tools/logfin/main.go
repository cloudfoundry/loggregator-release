package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
)

var (
	count     uint64
	instances uint64
)

func main() {
	inst, err := strconv.Atoi(os.Getenv("EMITTER_INSTANCES"))
	instances = uint64(inst)
	if instances == 0 || err != nil {
		log.Fatalf("No instances specified: %s", err)
	}

	http.HandleFunc("/", HealthCheckHandler)
	http.HandleFunc("/count", CountHandler)
	http.HandleFunc("/status", StatusHandler)

	err = http.ListenAndServe(":"+os.Getenv("PORT"), nil)
	if err != nil {
		log.Fatalf("Error listening: %s", err)
	}
}

func HealthCheckHandler(rw http.ResponseWriter, r *http.Request) {
	rw.WriteHeader(http.StatusOK)
}

func CountHandler(rw http.ResponseWriter, r *http.Request) {
	atomic.AddUint64(&count, 1)
	log.Printf("Incremented Counter to: %d", count)
	rw.WriteHeader(http.StatusOK)
}

func StatusHandler(rw http.ResponseWriter, r *http.Request) {
	if atomic.LoadUint64(&count) >= instances {
		log.Print("Yay! Heard from all apps")
		rw.WriteHeader(http.StatusOK)
		return
	}

	log.Print("Haven't heard from all apps yet")
	rw.WriteHeader(http.StatusPreconditionFailed)
}
