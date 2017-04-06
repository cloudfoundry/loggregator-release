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
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	cookieName := r.FormValue("CookieName")
	cookieValue := r.FormValue("CookieValue")
	origin := r.Header.Get("Origin")

	http.SetCookie(w, &http.Cookie{
		Name:   cookieName,
		Value:  cookieValue,
		Domain: h.domain,
		Secure: true,
	})

	w.Header().Add("Access-Control-Allow-Credentials", "true")
	w.Header().Add("Access-Control-Allow-Origin", origin)
}
