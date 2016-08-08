package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"tools/reports"
)

var (
	emittersReported uint32
	emitterInstances uint32
	counterInstances uint32
	countersReported uint32
	fullReport       = &reports.LogCount{
		Messages: make(map[string]*reports.MessageCount),
	}
)

type methodHandler map[string]http.Handler

func (m methodHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler, ok := m[r.Method]
	if !ok {
		for method := range m {
			w.Header().Add("Allow", method)
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	handler.ServeHTTP(w, r)
}

func main() {
	inst, err := strconv.Atoi(os.Getenv("EMITTER_INSTANCES"))
	emitterInstances = uint32(inst)
	if emitterInstances == 0 || err != nil {
		log.Fatalf("No emitter instances specified: %s", err)
	}
	inst, err = strconv.Atoi(os.Getenv("COUNTER_INSTANCES"))
	counterInstances = uint32(inst)
	if counterInstances == 0 || err != nil {
		log.Fatalf("No counter instances specified: %s", err)
	}

	http.HandleFunc("/", HealthCheckHandler)
	http.HandleFunc("/count", CountHandler)
	http.HandleFunc("/status", StatusHandler)
	http.Handle("/report", methodHandler{
		"POST": http.HandlerFunc(ReportCreator),
		"GET":  http.HandlerFunc(ReportSender),
	})

	err = http.ListenAndServe(":"+os.Getenv("PORT"), nil)
	if err != nil {
		log.Fatalf("Error listening: %s", err)
	}
}

func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func CountHandler(w http.ResponseWriter, r *http.Request) {
	atomic.AddUint32(&emittersReported, 1)
	log.Printf("Incremented Counter to: %d", emittersReported)
	w.WriteHeader(http.StatusOK)
}

func StatusHandler(w http.ResponseWriter, r *http.Request) {
	if atomic.LoadUint32(&emittersReported) >= emitterInstances {
		log.Print("Yay! Heard from all apps")
		w.WriteHeader(http.StatusOK)
		return
	}

	log.Print("Haven't heard from all apps yet")
	w.WriteHeader(http.StatusPreconditionFailed)
}

func ReportCreator(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var report reports.LogCount
	err := decoder.Decode(&report)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	log.Printf("Got back report %+v", report)
	fullReport.Add(report)

	scheme := "http"
	if r.TLS != nil {
		scheme += "s"
	}
	atomic.AddUint32(&countersReported, 1)
	w.Header().Set("Location", scheme+"://"+r.Host+r.URL.Path)
	w.WriteHeader(http.StatusCreated)
}

func ReportSender(w http.ResponseWriter, r *http.Request) {
	if atomic.LoadUint32(&countersReported) < counterInstances {
		w.WriteHeader(http.StatusPreconditionFailed)
		return
	}

	body, err := json.Marshal(fullReport)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(body)
}
