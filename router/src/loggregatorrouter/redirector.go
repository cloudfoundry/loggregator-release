package loggregatorrouter

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/servernamer"
	"net/http"
	"strings"
)

type redirector struct {
	host   string
	h      *hasher
	logger *gosteno.Logger
}

func NewRedirector(host string, h *hasher, logger *gosteno.Logger) (r *redirector) {
	r = &redirector{host: host, h: h, logger: logger}
	return
}

func (r *redirector) generateRedirectUrl(req *http.Request) string {
	req.ParseForm()
	server, _ := r.h.getLoggregatorServerForAppId(req.Form.Get("app"))

	uri := servernamer.ServerName(server, req.Host) + req.URL.RequestURI()

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

func (r *redirector) Start() (err error) {
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		urlStr := r.generateRedirectUrl(req)
		r.logger.Debugf("Redirector: client requested [%s], redirecting to [%s]", req.URL, urlStr)
		http.Redirect(w, req, urlStr, http.StatusFound)
	})

	err = http.ListenAndServe(r.host, nil)
	return err
}
