package syslogwriter_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"loggregator/sinks/syslogwriter"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var _ = Describe("SyslogWriter", func() {
	Context("With syslog Connection", func() {

		var dataChan <-chan []byte
		var serverStoppedChan <-chan bool
		var shutdownChan chan bool
		var sysLogWriter syslogwriter.SyslogWriter

		BeforeEach(func() {
			shutdownChan = make(chan bool)
			dataChan, serverStoppedChan = startSyslogServer(shutdownChan)
			outputUrl, _ := url.Parse("syslog://localhost:9999")
			sysLogWriter = syslogwriter.NewSyslogWriter(outputUrl, "appId", false)
			sysLogWriter.Connect()
		})

		AfterEach(func() {
			close(shutdownChan)
			sysLogWriter.Close()
			<-serverStoppedChan
		})

		Context("Message Format", func() {
			It("should send messages from stdout with INFO priority", func(done Done) {
				sysLogWriter.WriteStdout([]byte("just a test"), "test", "", time.Now().UnixNano())

				data := <-dataChan
				Expect(string(data)).To(MatchRegexp(`\d <14>\d `))
				close(done)
			})

			It("should send messages from stderr with ERROR priority", func(done Done) {
				sysLogWriter.WriteStderr([]byte("just a test"), "test", "", time.Now().UnixNano())

				data := <-dataChan
				Expect(string(data)).To(MatchRegexp(`\d <11>\d `))
				close(done)
			})

			It("should send messages in the proper format", func(done Done) {
				sysLogWriter.WriteStdout([]byte("just a test"), "App", "2", time.Now().UnixNano())

				data := <-dataChan
				Expect(string(data)).To(MatchRegexp(`\d <\d+>1 \d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}([-+]\d{2}:\d{2}) loggregator appId \[App/2\] - - just a test\n`))
				close(done)
			})

			It("should strip null termination char from message", func(done Done) {
				sysLogWriter.WriteStdout([]byte(string(0)+" hi"), "appId", "", time.Now().UnixNano())

				data := <-dataChan
				Expect(string(data)).ToNot(MatchRegexp("\000"))
				close(done)
			})
		})
	})

	Context("With TLS Connection", func() {

		var shutdownChan chan bool
		var serverStoppedChan <-chan bool

		BeforeEach(func() {
			shutdownChan = make(chan bool)
			serverStoppedChan = startTLSSyslogServer(shutdownChan, loggertesthelper.Logger())
		})

		AfterEach(func() {
			close(shutdownChan)
			<-serverStoppedChan
		})

		It("should connect", func() {
			outputUrl, _ := url.Parse("syslog-tls://localhost:9999")
			w := syslogwriter.NewSyslogWriter(outputUrl, "appId", true)
			err := w.Connect()
			Expect(err).To(BeNil())
			_, err = w.WriteStdout([]byte("just a test"), "test", "", time.Now().UnixNano())
			Expect(err).To(BeNil())
			w.Close()
		})

		It("should reject self-signed certs", func() {
			outputUrl, _ := url.Parse("syslog-tls://localhost:9999")
			w := syslogwriter.NewSyslogWriter(outputUrl, "appId", false)
			err := w.Connect()
			Expect(err).ToNot(BeNil())
		})
	})
})

type handler struct{}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
}

func startSyslogServer(shutdownChan <-chan bool) (<-chan []byte, <-chan bool) {
	dataChan := make(chan []byte)
	doneChan := make(chan bool)
	listener, err := net.Listen("tcp", "localhost:9999")
	if err != nil {
		panic(err)
	}

	go func() {
		<-shutdownChan
		listener.Close()
		close(doneChan)
	}()

	go func() {
		buffer := make([]byte, 1024)
		conn, err := listener.Accept()
		if err != nil {
			return
		}

		readCount, err := conn.Read(buffer)
		buffer2 := make([]byte, readCount)
		copy(buffer2, buffer[:readCount])
		dataChan <- buffer2
		conn.Close()
	}()

	<-time.After(300 * time.Millisecond)
	return dataChan, doneChan
}

func startTLSSyslogServer(shutdownChan <-chan bool, logger *gosteno.Logger) <-chan bool {
	doneChan := make(chan bool)
	generateCert(logger)
	cert, err := tls.LoadX509KeyPair("cert.pem", "key.pem")
	if err != nil {
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
		<-shutdownChan
		listener.Close()
		close(doneChan)
	}()

	go func() {
		buffer := make([]byte, 1024)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		_, err = conn.Read(buffer)

		conn.Close()
	}()

	<-time.After(300 * time.Millisecond)
	return doneChan
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
