package httpsetup

import (
	"crypto/tls"
	"net/http"
	"time"
)

var (
	transport *http.Transport
)

func init() {
	transport = &http.Transport{
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     &tls.Config{},
		DisableKeepAlives:   true,
	}

	http.DefaultClient.Transport = transport
	http.DefaultClient.Timeout = 20 * time.Second

}

func SetInsecureSkipVerify(skipCert bool) {
	transport.TLSClientConfig.InsecureSkipVerify = skipCert
}
