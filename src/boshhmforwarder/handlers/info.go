package handlers

import (
	"net/http"

	"fmt"
	"time"
)

type infoHandler struct {
	timeStarted time.Time
}

func NewInfoHandler() http.Handler {
	return &infoHandler{
		timeStarted: time.Now(),
	}
}

func (h *infoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	upTime := time.Since(h.timeStarted)

	bodyBytes := []byte(fmt.Sprintf(`{"uptime":"%s"}`, upTime))
	w.Write(bodyBytes)
}
