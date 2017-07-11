package proxy

import (
	"fmt"
	"net/http"

	"code.cloudfoundry.org/loggregator/trafficcontroller/internal/auth"
)

type AdminAccessMiddleware struct {
	authorize auth.AdminAccessAuthorizer
}

func NewAdminAccessMiddleware(a auth.AdminAccessAuthorizer) *AdminAccessMiddleware {
	return &AdminAccessMiddleware{authorize: a}
}

func (m *AdminAccessMiddleware) Wrap(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authToken := getAuthToken(r)

		authorized, err := m.authorize(authToken)
		if !authorized {
			w.Header().Set("WWW-Authenticate", "Basic")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintf(w, "You are not authorized. %s", err.Error())
			return
		}

		h.ServeHTTP(w, r)
	})
}
