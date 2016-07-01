package handlers

import (
	"boshhmforwarder/logging"
	"encoding/base64"
	"net/http"
	"net/http/pprof"
	"strings"

	"github.com/gorilla/mux"
)

func CreateDebugEndpoints(router *mux.Router, debugUser, debugPassword string) {
	logging.Log.Infof("Creating debug handlers")

	router.HandleFunc("/debug/pprof", authenticate(http.HandlerFunc(pprof.Index), debugUser, debugPassword))
	router.HandleFunc("/debug/pprof/cmdline", authenticate(http.HandlerFunc(pprof.Cmdline), debugUser, debugPassword))
	router.HandleFunc("/debug/pprof/profile", authenticate(http.HandlerFunc(pprof.Profile), debugUser, debugPassword))
	router.HandleFunc("/debug/pprof/symbol", authenticate(http.HandlerFunc(pprof.Symbol), debugUser, debugPassword))
	router.HandleFunc("/debug/pprof/goroutine", authenticate(pprof.Handler("goroutine").ServeHTTP, debugUser, debugPassword))
	router.HandleFunc("/debug/pprof/heap", authenticate(pprof.Handler("heap").ServeHTTP, debugUser, debugPassword))
	router.HandleFunc("/debug/pprof/threadcreate", authenticate(pprof.Handler("threadcreate").ServeHTTP, debugUser, debugPassword))
	router.HandleFunc("/debug/pprof/block", authenticate(pprof.Handler("block").ServeHTTP, debugUser, debugPassword))
}

func authenticate(h http.HandlerFunc, debugUser, debugPassword string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		doAuthentication(w, r, h, debugUser, debugPassword)
	}
}

func doAuthentication(w http.ResponseWriter, r *http.Request, innerHandler func(w http.ResponseWriter, r *http.Request), debugUser, debugPassword string) {
	w.Header().Set("WWW-Authenticate", `Basic realm="pprof"`)

	s := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
	if len(s) != 2 || s[0] != "Basic" {
		http.Error(w, "Invalid authorization header", 401)
		return
	}

	b, err := base64.StdEncoding.DecodeString(s[1])
	if err != nil {
		http.Error(w, err.Error(), 401)
		return
	}

	credentials := strings.SplitN(string(b), ":", 2)
	if len(credentials) != 2 {
		http.Error(w, "Invalid authorization header", 401)
		return
	}

	if !(debugUser == credentials[0] && debugPassword == credentials[1]) {
		http.Error(w, "Not authorized", 401)
		return
	}

	innerHandler(w, r)
}
