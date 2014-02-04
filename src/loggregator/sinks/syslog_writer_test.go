package sinks

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/stretchr/testify/assert"
	"math/big"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestTLSConnection(t *testing.T) {
	startTLSSyslogServer(loggertesthelper.Logger())

	w := NewSyslogWriter("syslog-tls", "localhost:9999", "appId", true)
	err := w.Connect()
	assert.NoError(t, err)
	_, err = w.write(4, "test", "just a test", "", time.Now().UnixNano())
	assert.NoError(t, err)
}

func TestTLSConnectionRejectsSelfSignedCertsByDefault(t *testing.T) {
	startTLSSyslogServer(loggertesthelper.Logger())

	w := NewSyslogWriter("syslog-tls", "localhost:9999", "appId", false)
	err := w.Connect()
	assert.Error(t, err)
}

type handler struct{}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
}

func startTLSSyslogServer(logger *gosteno.Logger) {
	generateCert(logger)
	cert, err := tls.LoadX509KeyPair("cert.pem", "key.pem")
	if err != nil {
		println("Err loading cert: ", err)
		panic(err)
	}
	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	listener, err := tls.Listen("tcp", "localhost:9999", config)

	if err != nil {
		panic(err)
	}
	go func() {
		for {
			buffer := make([]byte, 1024)
			conn, err := listener.Accept()
			if err != nil {
				panic(err)
			}
			_, err = conn.Read(buffer)

			conn.Close()
			listener.Close()
			return
		}
	}()

	<-time.After(300 * time.Millisecond)
}

func generateCert(logger *gosteno.Logger) {
	// see: http://golang.org/src/pkg/crypto/tls/generate_cert.go
	host := "localhost"
	validFrom := "Jan 1 15:04:05 2011"
	validFor := 10 * 365 * 24 * time.Hour
	isCA := true
	rsaBits := 1024

	if len(host) == 0 {
		logger.Fatalf("Missing required --host parameter")
	}

	priv, err := rsa.GenerateKey(rand.Reader, rsaBits)
	if err != nil {
		logger.Fatalf("failed to generate private key: %s", err)
		panic(err)
	}

	var notBefore time.Time
	if len(validFrom) == 0 {
		notBefore = time.Now()
	} else {
		notBefore, err = time.Parse("Jan 2 15:04:05 2006", validFrom)
		if err != nil {
			logger.Fatalf("Failed to parse creation date: %s\n", err)
			panic(err)
		}
	}

	notAfter := notBefore.Add(validFor)

	// end of ASN.1 time
	endOfTime := time.Date(2049, 12, 31, 23, 59, 59, 0, time.UTC)
	if notAfter.After(endOfTime) {
		notAfter = endOfTime
	}

	template := x509.Certificate{
		SerialNumber: new(big.Int).SetInt64(0),
		Subject: pkix.Name{
			Organization: []string{"Loggregator TrafficController TEST"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	hosts := strings.Split(host, ",")
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	if isCA {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		logger.Fatalf("Failed to create certificate: %s", err)
		panic(err)
	}

	certOut, err := os.Create("cert.pem")
	if err != nil {
		logger.Fatalf("failed to open cert.pem for writing: %s", err)
		panic(err)
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()
	logger.Info("written cert.pem\n")

	keyOut, err := os.OpenFile("key.pem", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		logger.Fatalf("failed to open key.pem for writing: %s", err)
		panic(err)
	}
	pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	keyOut.Close()
	logger.Info("written key.pem\n")
}
