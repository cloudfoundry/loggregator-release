package proxy

import (
	"net/http"
	"trafficcontroller/internal/auth"

	"github.com/gorilla/mux"
)

type LogAccessMiddleware struct {
	handler   http.Handler
	authorize auth.LogAccessAuthorizer
}

func NewLogAccessMiddleware(authorize auth.LogAccessAuthorizer, h http.Handler) *LogAccessMiddleware {
	return &LogAccessMiddleware{
		authorize: authorize,
		handler:   h,
	}
}

func (m *LogAccessMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	appID := mux.Vars(r)["appID"]
	authToken := getAuthToken(r)

	status, _ := m.authorize(authToken, appID)
	if status != http.StatusOK {
		switch status {
		case http.StatusUnauthorized:
			w.WriteHeader(status)
			w.Header().Set("WWW-Authenticate", "Basic")
		case http.StatusForbidden, http.StatusNotFound:
			status = http.StatusNotFound
		default:
			status = http.StatusInternalServerError
		}

		w.WriteHeader(status)

		return
	}

	m.handler.ServeHTTP(w, r)
}
