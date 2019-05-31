package proxy

import "net/http"

// CORSMiddleware provides support for adding CORS headers to OPTIONS
// requests.
type CORSMiddleware struct{}

// NewCORSMiddleware is the constructor for CORSMiddleware.
func NewCORSMiddleware() *CORSMiddleware {
	return &CORSMiddleware{}
}

// CORSOption is the type of all configuration options.
type CORSOption func(http.ResponseWriter)

// AllowCredentials enables credential sharing.
func AllowCredentials() CORSOption {
	return func(w http.ResponseWriter) {
		w.Header().Add("Access-Control-Allow-Credentials", "true")
	}
}

// AllowHeader configures a single allowed header. For multiple headers, pass
// in a comma-space deliminated string, e.g., "header-a, header-b".
func AllowHeader(header string) CORSOption {
	return func(w http.ResponseWriter) {
		w.Header().Add("Access-Control-Allow-Headers", header)
	}
}

// Wrap appends CORS headers to all responses.
func (CORSMiddleware) Wrap(h http.Handler, opts ...CORSOption) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		w.Header().Add("Access-Control-Allow-Origin", origin)

		for _, o := range opts {
			o(w)
		}

		h.ServeHTTP(w, r)
	})
}
