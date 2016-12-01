package plumbing

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
)

var supportedCipherSuites = []uint16{
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
}

// NewTLSConfig
func NewTLSConfig() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
		CipherSuites:       supportedCipherSuites,
	}
}

// NewMutualTLSConfig returns a tls.Config with certs loaded from files and
// the ServerName set.
func NewMutualTLSConfig(certFile, keyFile, caCertFile, serverName string) (*tls.Config, error) {
	tlsCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load keypair: %s", err.Error())
	}

	tlsConfig := NewTLSConfig()
	tlsConfig.Certificates = []tls.Certificate{tlsCert}
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	tlsConfig.ServerName = serverName

	if caCertFile != "" {
		certBytes, err := ioutil.ReadFile(caCertFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read ca cert file: %s", err.Error())
		}

		caCertPool := x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM(certBytes); !ok {
			return nil, errors.New("unable to load ca cert file")
		}
		tlsConfig.RootCAs = caCertPool
		tlsConfig.ClientCAs = caCertPool
	}

	return tlsConfig, nil
}
