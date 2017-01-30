package plumbing

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"log"

	"google.golang.org/grpc/credentials"
)

var supportedCipherSuites = []uint16{
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
}

type CASignatureError string

func (e CASignatureError) Error() string {
	return string(e)
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
		if err := addCA(tlsConfig, tlsCert, caCertFile); err != nil {
			return nil, err
		}
	}

	return tlsConfig, err
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

func NewCredentials(certFile, keyFile, caCertFile, serverName string) credentials.TransportCredentials {
	tlsConfig, err := NewMutualTLSConfig(
		certFile,
		keyFile,
		caCertFile,
		serverName,
	)

	if err != nil {
		log.Printf("failed to create mutual tls config: %v", err)
		return nil
	}

	return credentials.NewTLS(tlsConfig)
}
