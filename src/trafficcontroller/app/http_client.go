package app

import (
	"net/http"
	"time"

	"plumbing"
)

var (
	transport *http.Transport
)

func init() {
	tlsConf := plumbing.NewTLSConfig()
	transport = &http.Transport{
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     tlsConf,
		DisableKeepAlives:   true,
	}

	http.DefaultClient.Transport = transport
	http.DefaultClient.Timeout = 20 * time.Second

}

func SetInsecureSkipVerify(skipCert bool) {
	transport.TLSClientConfig.InsecureSkipVerify = skipCert
}
