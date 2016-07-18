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
	var err error
	var inst int
	inst, err = strconv.Atoi(os.Getenv("INSTANCES"))

	instances = uint64(inst)
	if instances == 0 || err != nil {
		log.Fatalf("No instances specified: %s", err)
	}

	countHandler := &Counter{}
	statusHandler := &Status{}

	http.Handle("/count", countHandler)
	http.Handle("/status", statusHandler)

	err = http.ListenAndServe(":"+os.Getenv("PORT"), nil)
	if err != nil {
		log.Fatalf("Error listening: %s", err)
	}
}

type Counter struct {
}

func (c *Counter) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	atomic.AddUint64(&count, 1)
	log.Printf("Incremented Counter to: %d", count)
	return
}

type Status struct {
}

func (c *Status) ServeHTTP(rw http.ResponseWriter, r *http.Request) {

	if atomic.LoadUint64(&count) >= instances {
		log.Print("Yay! Heard from all apps")
		rw.WriteHeader(http.StatusOK)
		return
	}
	log.Print("Haven't heard from all apps yet")
	rw.WriteHeader(http.StatusPreconditionFailed)
	return

}
