package varz

import (
	"encoding/json"
	"fmt"
	mbus "github.com/cloudfoundry/go_cfmessagebus"
	"net/http"
	"runtime"
	"time"
)

var Component VcapComponent
var healthz *Healthz
var varz *Varz

var procStat *ProcessStatus

type VcapComponent struct {
	// These fields are from individual components
	Type        string                    `json:"type"`
	Index       uint                      `json:"index"`
	Host        string                    `json:"host"`
	Credentials []string                  `json:"credentials"`
	Config      interface{}               `json:"config"`
	Varz        *Varz                     `json:"-"`
	Healthz     *Healthz                  `json:"-"`
	InfoRoutes  map[string]json.Marshaler `json:"-"`

	// These fields are automatically generated
	UUID   string   `json:"uuid"`
	Start  time.Time     `json:"start"`
	Uptime time.Duration `json:"uptime"`
}

func UpdateHealthz() *Healthz {
	// lock and unlock immediately to determine if we are in deadlock state
	healthz.LockableObject.Lock()
	healthz.LockableObject.Unlock()
	return healthz
}

func UpdateVarz() *Varz {
	varz.Lock()
	defer varz.Unlock()

	varz.MemStat = procStat.MemRss
	varz.Cpu = procStat.CpuUsage
	varz.Uptime = time.Now().Sub(Component.Start)

	return varz
}

func StartComponent(c *VcapComponent) {
	Component = *c
	if Component.Type == "" {
		panic("type is required")
	}

	Component.Start = time.Now()
	Component.UUID = fmt.Sprintf("%d-%s", Component.Index, generateUUID())

	if Component.Host == "" {
		host, err := localIP()
		if err != nil {
			panic(err)
		}

		port, err := grabEphemeralPort()
		if err != nil {
			panic(err)
		}

		Component.Host = fmt.Sprintf("%s:%s", host, port)
	}

	if Component.Credentials == nil || len(Component.Credentials) != 2 {
		user := generateUUID()
		password := generateUUID()

		Component.Credentials = []string{user, password}
	}

	varz = Component.Varz
	varz.NumCores = runtime.NumCPU()

	procStat = NewProcessStatus()

	healthz = Component.Healthz

	go c.ListenAndServe()
}

func Register(c *VcapComponent, mbusClient mbus.CFMessageBus) {
	mbusClient.RespondToChannel("vcap.component.discover", func(payload []byte) []byte {
		Component.Uptime = time.Now().Sub(Component.Start)
		b, e := json.Marshal(Component)
		if e != nil {
			return nil
		}
		return b
	})

	b, e := json.Marshal(Component)
	if e != nil {
		panic("Component's information should be correct")
	}
	mbusClient.Publish("vcap.component.announce", b)
}

func (c *VcapComponent) ListenAndServe() {
	hs := http.NewServeMux()

	hs.HandleFunc("/healthz", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)

		fmt.Fprintf(w, UpdateHealthz().Value())
	})

	hs.HandleFunc("/varz", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		enc := json.NewEncoder(w)
		enc.Encode(UpdateVarz())
	})

	for path, marshaler := range c.InfoRoutes {
		hs.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			enc := json.NewEncoder(w)
			enc.Encode(marshaler)
		})
	}

	f := func(user, password string) bool {
		return user == c.Credentials[0] && password == c.Credentials[1]
	}

	s := &http.Server{
		Addr:    c.Host,
		Handler: &BasicAuth{hs, f},
	}

	err := s.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
