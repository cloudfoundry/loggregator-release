package httpsetup

import (
	"crypto/tls"
	"net/http"
	"plumbing"
	"time"
)

var (
	transport *http.Transport
)

func init() {
	transport = &http.Transport{
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig: &tls.Config{
			CipherSuites: plumbing.SupportedCipherSuites,
			MinVersion:   tls.VersionTLS12,
		},
		DisableKeepAlives: true,
	}

	http.DefaultClient.Transport = transport
	http.DefaultClient.Timeout = 20 * time.Second

}

func SetInsecureSkipVerify(skipCert bool) {
	transport.TLSClientConfig.InsecureSkipVerify = skipCert
}
