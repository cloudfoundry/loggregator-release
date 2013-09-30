package loggregatorrouter

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/servernamer"
	"loggregatorrouter/hasher"
	"net/http"
)

type redirector struct {
	host   string
	h      *hasher.Hasher
	logger *gosteno.Logger
}

func NewRedirector(host string, h *hasher.Hasher, logger *gosteno.Logger) (r *redirector) {
	r = &redirector{host: host, h: h, logger: logger}
	return
}

func (r *redirector) generateRedirectUrl(req *http.Request) string {
	req.ParseForm()
	server, _ := r.h.GetLoggregatorServerForAppId(req.Form.Get("app"))

	uri := servernamer.ServerName(server, req.Host) + req.URL.RequestURI()

	var proto string
	reqProto := req.Header.Get("X-Forwarded-Proto")

	if reqProto == "http" {
		proto = "ws://"
	} else {
		proto = "wss://"
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
