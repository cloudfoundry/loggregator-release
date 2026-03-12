package proxy

import "net/http"

type SetCookieHandler struct {
	domain string
}

func NewSetCookieHandler(domain string) *SetCookieHandler {
	return &SetCookieHandler{
		domain: domain,
	}
}

func (h SetCookieHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Body != nil {
		r.Body = http.MaxBytesReader(w, r.Body, 4096) // limit request body to 4kB
	}
	err := r.ParseForm()
	if err != nil || r.FormValue("CookieName") == "" || r.FormValue("CookieValue") == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	cookieName := r.FormValue("CookieName")
	cookieValue := r.FormValue("CookieValue")
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    cookieValue,
		Domain:   h.domain,
		Secure:   true,
		HttpOnly: true,
	})
}
