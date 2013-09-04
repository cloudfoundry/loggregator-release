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
		// X-Forwarded-Proto is set for all https and http requests
		proto = proto + "://"
		r.logger.Debugf("Using X-Forwarded-Proto value %v for redirect protocol", proto)
	} else if strings.Contains(req.Host, ":4443") {
		proto = "wss://"
		r.logger.Debug("Using wss protocol because request port was 4443")
	} else {
		proto = "ws://"
		r.logger.Debug("Falling back to ws protocol")
	}

	return proto + uri
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
