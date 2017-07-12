package plumbing

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"

	"google.golang.org/grpc/credentials"
)

var defaultCipherSuites = []uint16{
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
}

type CASignatureError string

func (e CASignatureError) Error() string {
	return string(e)
}

func NewTLSConfig() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
	}
}

var cipherMap = map[string]uint16{
	"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256": tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384": tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
}

type ConfigOption func(*tls.Config)

func WithCipherSuites(ciphers []string) ConfigOption {
	return func(c *tls.Config) {
		var configuredCiphers []uint16
		for _, c := range ciphers {
			cipher, ok := cipherMap[c]
			if !ok {
				continue
			}
			configuredCiphers = append(configuredCiphers, cipher)
		}
		c.CipherSuites = configuredCiphers
		if len(c.CipherSuites) == 0 {
			panic("no valid ciphers provided for TLS configuration")
		}
	}
}

// NewClientMutualTLSConfig returns a tls.Config with certs loaded from files and
// the ServerName set.
func NewClientMutualTLSConfig(
	certFile string,
	keyFile string,
	caCertFile string,
	serverName string,
) (*tls.Config, error) {
	return newMutualTLSConfig(
		certFile,
		keyFile,
		caCertFile,
		serverName,
	)
}

// NewServerMutualTLSConfig returns a tls.Config with certs loaded from files and
// the ServerName set. The returned tls.Config has configured list of cipher
// suites.
func NewServerMutualTLSConfig(
	certFile string,
	keyFile string,
	caCertFile string,
	serverName string,
	opts ...ConfigOption,
) (*tls.Config, error) {
	tlsConfig, err := newMutualTLSConfig(
		certFile,
		keyFile,
		caCertFile,
		serverName,
	)
	if err != nil {
		return nil, err
	}

	tlsConfig.CipherSuites = defaultCipherSuites
	for _, opt := range opts {
		opt(tlsConfig)
	}

	return tlsConfig, nil
}

func NewClientCredentials(
	certFile string,
	keyFile string,
	caCertFile string,
	serverName string,
) (credentials.TransportCredentials, error) {
	tlsConfig, err := NewClientMutualTLSConfig(
		certFile,
		keyFile,
		caCertFile,
		serverName,
	)
	if err != nil {
		return nil, err
	}

	return credentials.NewTLS(tlsConfig), nil
}

func NewServerCredentials(
	certFile string,
	keyFile string,
	caCertFile string,
	serverName string,
	opts ...ConfigOption,
) (credentials.TransportCredentials, error) {
	tlsConfig, err := NewServerMutualTLSConfig(
		certFile,
		keyFile,
		caCertFile,
		serverName,
	)
	if err != nil {
		return nil, err
	}

	return credentials.NewTLS(tlsConfig), nil
}

func addCA(tlsConfig *tls.Config, tlsCert tls.Certificate, caCertFile string) error {
	certBytes, err := ioutil.ReadFile(caCertFile)
	if err != nil {
		return fmt.Errorf("failed to read ca cert file: %s", err.Error())
	}

	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM(certBytes); !ok {
		return errors.New("unable to load ca cert file")
	}
	tlsConfig.RootCAs = caCertPool
	tlsConfig.ClientCAs = caCertPool

	verifier, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		return err
	}

	verify_options := x509.VerifyOptions{
		Roots: caCertPool,
		KeyUsages: []x509.ExtKeyUsage{
			x509.ExtKeyUsageAny,
		},
	}
	_, err = verifier.Verify(verify_options)
	if err != nil {
		return CASignatureError(err.Error())
	}
	return nil
}

func newMutualTLSConfig(certFile, keyFile, caCertFile, serverName string) (*tls.Config, error) {
	tlsCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load keypair: %s", err.Error())
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
	}

	tlsConfig.Certificates = []tls.Certificate{tlsCert}
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	tlsConfig.ServerName = serverName

	if caCertFile != "" {
		if err := addCA(tlsConfig, tlsCert, caCertFile); err != nil {
			return nil, err
		}
	}

	return tlsConfig, err
}
