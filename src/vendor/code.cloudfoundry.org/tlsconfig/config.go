// Package tlsconfig provides opintionated helpers for building tls.Configs.
// It keeps up to date with internal CloudFoundry best practices and external
// industry best practices.
package tlsconfig

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
)

// Config represents a half configured TLS configuration. It can be made usable
// by calling either of its two methods.
type Config struct {
	opts []TLSOption
}

// TLSOption can be used to configure a TLS configuration for both clients and
// servers.
type TLSOption func(*tls.Config) error

// ServerOption can be used to configure a TLS configuration for a server.
type ServerOption func(*tls.Config) error

// ClientOption can be used to configure a TLS configuration for a client.
type ClientOption func(*tls.Config) error

// Build creates a half configured TLS configuration.
func Build(opts ...TLSOption) Config {
	return Config{
		opts: opts,
	}
}

// Server can be used to build a TLS configuration suitable for servers (GRPC,
// HTTP, etc.). The options are applied in order. It is possible for a later
// option to undo the configuration that an earlier one applied. Care must be
// taken.
func (c Config) Server(opts ...ServerOption) (*tls.Config, error) {
	config := &tls.Config{}

	for _, opt := range c.opts {
		if err := opt(config); err != nil {
			return nil, err
		}
	}

	for _, opt := range opts {
		if err := opt(config); err != nil {
			return nil, err
		}
	}

	return config, nil
}

// Client can be used to build a TLS configuration suitable for clients (GRPC,
// HTTP, etc.). The options are applied in order. It is possible for a later
// option to undo the configuration that an earlier one applied. Care must be
// taken.
func (c Config) Client(opts ...ClientOption) (*tls.Config, error) {
	config := &tls.Config{}

	for _, opt := range c.opts {
		if err := opt(config); err != nil {
			return nil, err
		}
	}

	for _, opt := range opts {
		if err := opt(config); err != nil {
			return nil, err
		}
	}

	return config, nil
}

// WithInternalServiceDefaults modifies a *tls.Config that is suitable for use
// in communication links between internal services. It is not guaranteed to be
// suitable for communication to other external services as it contains a
// strict definition of acceptable standards.
//
// The standards were taken from the "Consolidated Remarks" internal document
// from Pivotal. The one exception to this is the use of the P256 curve in
// order to support gRPC clients which hardcode this configuration.
//
// Note: Due to the aggressive nature of the ciphersuites chosen here (they do
// not support any ECC signing) it is not possible to use ECC keys with this
// option.
func WithInternalServiceDefaults() TLSOption {
	return func(c *tls.Config) error {
		c.MinVersion = tls.VersionTLS12
		c.PreferServerCipherSuites = true
		c.CipherSuites = []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		}
		return nil
	}
}

// WithIdentity sets the identity of the server or client which will be
// presented to its peer upon connection.
func WithIdentity(cert tls.Certificate) TLSOption {
	return func(c *tls.Config) error {
		fail := func(err error) error {
			return fmt.Errorf("failed to load keypair: %s", err.Error())
		}
		c.Certificates = []tls.Certificate{cert}
		x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return fail(err)
		}
		err = checkExpiration(x509Cert)
		if err != nil {
			return fail(err)
		}
		return nil
	}
}

// WithIdentityFromFile sets the identity of the server or client which will be
// presented to its peer upon connection from provided cert and key files.
func WithIdentityFromFile(certPath string, keyPath string) TLSOption {
	return func(c *tls.Config) error {
		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			return fmt.Errorf("failed to load keypair: %s", err.Error())
		}
		return WithIdentity(cert)(c)
	}
}

// WithClientAuthentication makes the server verify that all clients present an
// identity that can be validated by the certificate pool provided.
func WithClientAuthentication(authority *x509.CertPool) ServerOption {
	return func(c *tls.Config) error {
		c.ClientAuth = tls.RequireAndVerifyClientCert
		c.ClientCAs = authority
		return nil
	}
}

// WithClientAuthenticationFromFile makes the server verify that all clients present an
// identity that can be validated by the CA file provided.
func WithClientAuthenticationFromFile(caPath string) ServerOption {
	return func(c *tls.Config) error {
		caBytes, err := ioutil.ReadFile(caPath)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %s", caPath, err.Error())
		}

		caCertPool := x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM(caBytes); !ok {
			return fmt.Errorf("unable to load CA certificate at %s", caPath)
		}

		return WithClientAuthentication(caCertPool)(c)
	}
}

// WithAuthority makes the client verify that the server presents an identity
// that can be validated by the certificate pool provided.
func WithAuthority(authority *x509.CertPool) ClientOption {
	return func(c *tls.Config) error {
		c.RootCAs = authority
		return nil
	}
}

// WithAuthorityFromFile makes the client verify that the server presents an identity
// that can be validated by the CA file provided.
func WithAuthorityFromFile(caPath string) ClientOption {
	return func(c *tls.Config) error {
		caBytes, err := ioutil.ReadFile(caPath)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %s", caPath, err.Error())
		}

		caCertPool := x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM(caBytes); !ok {
			return fmt.Errorf("unable to load CA certificate at %s", caPath)
		}

		c.RootCAs = caCertPool
		return nil
	}
}

// WithServerName makes the client verify that the server name in the
// certificate presented by the server.
func WithServerName(name string) ClientOption {
	return func(c *tls.Config) error {
		c.ServerName = name
		return nil
	}
}
