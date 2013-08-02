package cfcomponent

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Component struct {
	sync.RWMutex
	IpAddress         string
	HealthMonitor     HealthMonitor
	SystemDomain      string
	WebPort           uint32
	RegisterInterval  time.Duration `json:"minimumRegisterIntervalInSeconds"`
	Type              string        //Used by the collector to find data processing class
	Index             uint
	UUID              string
	StatusPort        uint32
	StatusCredentials []string
}

func (c Component) StartHealthz() {
	handler := func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		if c.HealthMonitor.Ok() {
			fmt.Fprintf(w, "ok")
		} else {
			fmt.Fprintf(w, "bad")
		}
	}

	http.HandleFunc("/healtz", handler)

	go http.ListenAndServe(fmt.Sprintf("%s:%d", c.IpAddress, c.StatusPort), nil)
}
