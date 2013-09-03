package loggregatorrouter

import (
	"github.com/cloudfoundry/gosteno"
	"net/http"
	"strings"
)

type redirector struct {
	host string
	h    *hasher
}

func NewRedirector(host string, h *hasher, logger *gosteno.Logger) (r *redirector) {
	r = &redirector{host: host, h: h}
	return
}

func (r *redirector) generateRedirectUrl(req *http.Request) string {
	req.ParseForm()
	server, _ := r.h.getLoggregatorServerForAppId(req.Form.Get("app"))
	uri := strings.Replace(server, ".", "-", -1)
	uri = strings.Replace(uri, ":", "-", -1)
	uri = uri + "-" + req.Host + req.URL.RequestURI()

	var proto string
	if proto = req.Header.Get("X-Forwarded-Proto"); proto != "" {
		proto = proto + "://" // if the reverse proxy set the header, just use it
	} else if strings.Contains(req.Host, ":4443") {
		proto = "https://" // otherwise, we assume https if port 4443 is specified
	} else {
		proto = "http://" // default to http
	}

	uri = proto + uri

	return uri
}

func (r *redirector) Start() {
	var err error

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		urlStr := r.generateRedirectUrl(req)
		http.Redirect(w, req, urlStr, http.StatusFound)
	})

	err = http.ListenAndServe(r.host, nil)
	if err != nil {
		panic(err)
	}
}
