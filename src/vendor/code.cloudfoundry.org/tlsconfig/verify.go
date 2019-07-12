package tlsconfig

import (
	"crypto/x509"
	"errors"
	"time"
)

func checkExpiration(cert *x509.Certificate) error {
	sinceStart := time.Now().Sub(cert.NotBefore)
	untilExpiry := cert.NotAfter.Sub(time.Now())
	if untilExpiry < 0 || sinceStart < 0 {
		return errors.New("the certificate has expired or is not yet valid")
	}
	return nil
}
