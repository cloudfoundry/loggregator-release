package auth_test

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	. "github.com/onsi/gomega"
)

func newServerRequest(method, uri string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, uri, body)
	if err != nil {
		return nil, err
	}
	req.Host = req.URL.Host
	// set req.TLS if request is https
	if req.URL.Scheme == "https" {
		req.TLS = &tls.ConnectionState{}
	}
	// zero out fields that are not available to the server
	req.URL = &url.URL{
		Path:     req.URL.Path,
		RawQuery: req.URL.RawQuery,
	}
	return req, nil
}

func buildRequest(method, url, remoteAddr, requestId, forwardedFor string) *http.Request {
	req, err := newServerRequest(method, url, nil)
	Expect(err).ToNot(HaveOccurred())
	req.Header.Add("X-Vcap-Request-ID", requestId)
	req.Header.Add("X-Forwarded-For", forwardedFor)
	req.RemoteAddr = remoteAddr
	return req
}

func buildExpectedLog(timestamp time.Time, requestId, method, path, sourceHost, sourcePort, dstHost, dstPort string) string {
	extensions := []string{
		fmt.Sprintf("rt=%d", timestamp.UnixNano()/int64(time.Millisecond)),
		"cs1Label=userAuthenticationMechanism",
		"cs1=oauth-access-token",
		"cs2Label=vcapRequestId",
		"cs2=" + requestId,
		"request=" + path,
		"requestMethod=" + method,
		"src=" + sourceHost,
		"spt=" + sourcePort,
		"dst=" + dstHost,
		"dpt=" + dstPort,
	}
	fields := []string{
		"0",
		"cloud_foundry",
		"loggregator_trafficcontroller",
		"1.0",
		method + " " + path,
		method + " " + path,
		"0",
		strings.Join(extensions, " "),
	}
	return "CEF:" + strings.Join(fields, "|")
}
