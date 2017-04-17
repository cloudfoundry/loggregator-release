package httpsetup

import (
	"net/http"
	"plumbing"
	"time"
)

var (
	transport *http.Transport
)

func Setup(cipherSuites []string) {
	tlsConf := plumbing.NewTLSConfig(
		plumbing.WithCipherSuites(cipherSuites),
	)
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
