package app_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"code.cloudfoundry.org/loggregator-release/src/internal/testhelper"

	"code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator-release/src/plumbing" // TODO: Resolve duplicate proto error
	"code.cloudfoundry.org/loggregator-release/src/rlp-gateway/app"
	"code.cloudfoundry.org/loggregator-release/src/testservers"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"

	"code.cloudfoundry.org/tlsconfig/certtest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Gateway", func() {
	var (
		logsProvider *stubLogsProvider
		cfg          app.Config

		logger  = log.New(GinkgoWriter, "", log.LstdFlags)
		metrics = testhelper.NewMetricClient()

		localhostCerts = testservers.GenerateCerts("localhost")

		internalLogAccess *httptest.Server
		logAdmin          *httptest.Server
	)

	BeforeEach(func() {
		cfg = app.Config{
			LogsProviderCAPath:         testservers.LoggregatorTestCerts.CA(),
			LogsProviderClientCertPath: testservers.LoggregatorTestCerts.Cert("rlpgateway"),
			LogsProviderClientKeyPath:  testservers.LoggregatorTestCerts.Key("rlpgateway"),
			LogsProviderCommonName:     "reverselogproxy",

			StreamTimeout: 10 * time.Minute,

			HTTP: app.HTTP{
				GatewayAddr: ":0",
				CertPath:    localhostCerts.Cert("localhost"),
				KeyPath:     localhostCerts.Key("localhost"),
			},

			LogAccessAuthorization: app.LogAccessAuthorization{
				CertPath:   testservers.LoggregatorTestCerts.Cert("capi"),
				KeyPath:    testservers.LoggregatorTestCerts.Key("capi"),
				CAPath:     testservers.LoggregatorTestCerts.CA(),
				CommonName: "capi",
			},

			MetricsServer: app.MetricsServer{
				CertFile: testservers.LoggregatorTestCerts.Cert("metrics"),
				KeyFile:  testservers.LoggregatorTestCerts.Key("metrics"),
				CAFile:   testservers.LoggregatorTestCerts.CA(),
			},

			LogAdminAuthorization: app.LogAdminAuthorization{
				ClientID:     "client",
				ClientSecret: "secret",
				CAPath:       testservers.LoggregatorTestCerts.CA(),
			},
		}
	})

	AfterEach(func() {
		internalLogAccess.Close()
		logAdmin.Close()
		logsProvider.listener.Close()
	})

	Context("with valid log access authorization", func() {
		BeforeEach(func() {
			internalLogAccess = newAuthorizationServer(
				http.StatusOK,
				"{}",
				testservers.LoggregatorTestCerts.Cert("capi"),
				testservers.LoggregatorTestCerts.Key("capi"),
			)
			logAdmin = newAuthorizationServer(
				http.StatusBadRequest,
				invalidTokenResponse,
				testservers.LoggregatorTestCerts.Cert("uaa"),
				testservers.LoggregatorTestCerts.Key("uaa"),
			)

			logsProvider = newStubLogsProvider(testservers.LoggregatorTestCerts)
			logsProvider.toSend = 10

			cfg.LogsProviderAddr = logsProvider.addr()
			cfg.LogAccessAuthorization.Addr = internalLogAccess.URL
			cfg.LogAdminAuthorization.Addr = logAdmin.URL
		})

		It("forwards requests to RLP", func() {
			gateway := app.NewGateway(cfg, metrics, logger, GinkgoWriter)
			gateway.Start(false)
			defer gateway.Stop()

			client := newTestClient(&tls.Config{InsecureSkipVerify: true}) //nolint:gosec
			go func() {
				defer GinkgoRecover()
				resp, err := client.open("https://" + gateway.Addr() + "/v2/read?log&source_id=deadbeef-dead-dead-dead-deaddeafbeef")
				Expect(err).To(MatchError("EOF"))
				if resp != nil {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))
				}
			}()

			Eventually(client.envelopes).Should(HaveLen(10))
		})

		It("doesn't panic when the logs provider closes", func() {
			logsProvider.toSend = 1

			gateway := app.NewGateway(cfg, metrics, logger, GinkgoWriter)
			gateway.Start(false)
			defer gateway.Stop()

			client := newTestClient(&tls.Config{InsecureSkipVerify: true}) //nolint:gosec
			Expect(func() {
				_, err := client.open("https://" + gateway.Addr() + "/v2/read?log&source_id=deadbeef-dead-dead-dead-deaddeafbeef")
				Expect(err).To(MatchError("EOF"))
			}).ToNot(Panic())
		})

		It("does not accept unencrypted connections", func() {
			gateway := app.NewGateway(cfg, metrics, logger, GinkgoWriter)
			gateway.Start(false)
			defer gateway.Stop()

			client := newTestClient(&tls.Config{InsecureSkipVerify: true}) //nolint:gosec
			resp, err := client.open("http://" + gateway.Addr() + "/v2/read?log&source_id=deadbeef")
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
		})
	})

	Context("when the user is an admin", func() {
		BeforeEach(func() {
			internalLogAccess = newAuthorizationServer(
				http.StatusBadRequest,
				"{}",
				testservers.LoggregatorTestCerts.Cert("capi"),
				testservers.LoggregatorTestCerts.Key("capi"),
			)
			logAdmin = newAuthorizationServer(
				http.StatusOK,
				validTokenResponse,
				testservers.LoggregatorTestCerts.Cert("uaa"),
				testservers.LoggregatorTestCerts.Key("uaa"),
			)

			logsProvider = newStubLogsProvider(testservers.LoggregatorTestCerts)
			logsProvider.toSend = 10

			cfg.LogsProviderAddr = logsProvider.addr()
			cfg.LogAccessAuthorization.Addr = internalLogAccess.URL
			cfg.LogAdminAuthorization.Addr = logAdmin.URL
		})

		It("forwards HTTP requests to RLP", func() {
			gateway := app.NewGateway(cfg, metrics, logger, GinkgoWriter)
			gateway.Start(false)
			defer gateway.Stop()

			client := newTestClient(&tls.Config{InsecureSkipVerify: true}) //nolint:gosec
			go func() {
				defer GinkgoRecover()
				resp, err := client.open("https://" + gateway.Addr() + "/v2/read?log&source_id=deadbeef-dead-dead-dead-deaddeafbeef")
				Expect(err).To(MatchError("EOF"))
				if resp != nil {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))
				}
			}()

			Eventually(client.envelopes).Should(HaveLen(10))
		})

		It("doesn't panic when the logs provider closes", func() {
			logsProvider.toSend = 1

			gateway := app.NewGateway(cfg, metrics, logger, GinkgoWriter)
			gateway.Start(false)
			defer gateway.Stop()

			client := newTestClient(&tls.Config{InsecureSkipVerify: true}) //nolint:gosec
			Expect(func() {
				_, err := client.open("https://" + gateway.Addr() + "/v2/read?log")
				Expect(err).To(MatchError("EOF"))
			}).ToNot(Panic())
		})
	})

	Context("when the user does not have log access", func() {
		BeforeEach(func() {
			internalLogAccess = newAuthorizationServer(
				http.StatusBadRequest,
				"{}",
				testservers.LoggregatorTestCerts.Cert("capi"),
				testservers.LoggregatorTestCerts.Key("capi"),
			)
			logAdmin = newAuthorizationServer(
				http.StatusBadRequest,
				invalidTokenResponse,
				testservers.LoggregatorTestCerts.Cert("uaa"),
				testservers.LoggregatorTestCerts.Key("uaa"),
			)

			logsProvider = newStubLogsProvider(testservers.LoggregatorTestCerts)

			cfg.LogsProviderAddr = logsProvider.addr()
			cfg.LogAccessAuthorization.Addr = internalLogAccess.URL
			cfg.LogAdminAuthorization.Addr = logAdmin.URL
		})

		It("returns 404 if user is not authorized for log access for a source ID", func() {
			gateway := app.NewGateway(cfg, metrics, logger, GinkgoWriter)
			gateway.Start(false)
			defer gateway.Stop()

			client := newTestClient(&tls.Config{InsecureSkipVerify: true}) //nolint:gosec
			resp, err := client.open("https://" + gateway.Addr() + "/v2/read?log&source_id=some-id")
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})

		It("returns 404 if user is not authorized as admin", func() {
			gateway := app.NewGateway(cfg, metrics, logger, GinkgoWriter)
			gateway.Start(false)
			defer gateway.Stop()

			client := newTestClient(&tls.Config{InsecureSkipVerify: true}) //nolint:gosec
			resp, err := client.open("https://" + gateway.Addr() + "/v2/read?log")
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})
	})

	Describe("TLS security", func() {
		tlsConfigTest := func(tlsConfig *tls.Config) {
			client := newTestClient(tlsConfig)

			internalLogAccess = newAuthorizationServer(
				http.StatusOK,
				"{}",
				testservers.LoggregatorTestCerts.Cert("capi"),
				testservers.LoggregatorTestCerts.Key("capi"),
			)
			logAdmin = newAuthorizationServer(
				http.StatusBadRequest,
				invalidTokenResponse,
				testservers.LoggregatorTestCerts.Cert("uaa"),
				testservers.LoggregatorTestCerts.Key("uaa"),
			)

			logsProvider = newStubLogsProvider(testservers.LoggregatorTestCerts)
			logsProvider.toSend = 1

			cfg.LogsProviderAddr = logsProvider.addr()
			cfg.LogAccessAuthorization.Addr = internalLogAccess.URL
			cfg.LogAdminAuthorization.Addr = logAdmin.URL

			cfg.LogsProviderAddr = logsProvider.addr()
			gateway := app.NewGateway(cfg, metrics, logger, GinkgoWriter)
			gateway.Start(false)
			defer gateway.Stop()

			_, err := client.open("https://" + gateway.Addr() + "/v2/read?log&source_id=deadbeef-dead-dead-dead-deaddeafbeef")
			Expect(err).To(MatchError("EOF"))
		}

		DescribeTable("allows only supported TLS versions", func(clientTLSVersion int, serverShouldAllow bool) {
			runTest := func() {
				tlsConfigTest(buildTLSConfig(uint16(clientTLSVersion), tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256))
			}

			if serverShouldAllow {
				Expect(runTest).ToNot(Panic())
			} else {
				Expect(runTest).To(Panic())
			}
		},
			Entry("unsupported SSL 3.0", tls.VersionSSL30, false), //nolint:staticcheck
			Entry("unsupported TLS 1.0", tls.VersionTLS10, false),
			Entry("unsupported TLS 1.1", tls.VersionTLS11, false),
			Entry("supported TLS 1.2", tls.VersionTLS12, true),
		)

		DescribeTable("allows only supported cipher suites", func(clientCipherSuite uint16, serverShouldAllow bool) {
			runTest := func() {
				tlsConfigTest(buildTLSConfig(tls.VersionTLS12, clientCipherSuite))
			}

			if serverShouldAllow {
				Expect(runTest).ToNot(Panic())
			} else {
				Expect(runTest).To(Panic())
			}
		},
			Entry("unsupported cipher RSA_WITH_3DES_EDE_CBC_SHA", tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA, false),
			Entry("unsupported cipher ECDHE_RSA_WITH_3DES_EDE_CBC_SHA", tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA, false),
			Entry("unsupported cipher RSA_WITH_RC4_128_SHA", tls.TLS_RSA_WITH_RC4_128_SHA, false),
			Entry("unsupported cipher RSA_WITH_AES_128_CBC_SHA256", tls.TLS_RSA_WITH_AES_128_CBC_SHA256, false),
			Entry("unsupported cipher ECDHE_ECDSA_WITH_CHACHA20_POLY1305", tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305, false),
			Entry("unsupported cipher ECDHE_ECDSA_WITH_RC4_128_SHA", tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA, false),
			Entry("unsupported cipher ECDHE_ECDSA_WITH_AES_128_CBC_SHA", tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA, false),
			Entry("unsupported cipher ECDHE_ECDSA_WITH_AES_256_CBC_SHA", tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA, false),
			Entry("unsupported cipher ECDHE_ECDSA_WITH_AES_128_CBC_SHA256", tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256, false),
			Entry("unsupported cipher ECDHE_ECDSA_WITH_AES_128_GCM_SHA256", tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256, false),
			Entry("unsupported cipher ECDHE_ECDSA_WITH_AES_256_GCM_SHA384", tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384, false),
			Entry("unsupported cipher ECDHE_RSA_WITH_RC4_128_SHA", tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA, false),
			Entry("unsupported cipher ECDHE_RSA_WITH_AES_128_CBC_SHA256", tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256, false),
			Entry("unsupported cipher ECDHE_RSA_WITH_AES_128_CBC_SHA", tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA, false),
			Entry("unsupported cipher ECDHE_RSA_WITH_AES_256_CBC_SHA", tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA, false),
			Entry("unsupported cipher ECDHE_RSA_WITH_CHACHA20_POLY1305", tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305, false),
			Entry("unsupported cipher RSA_WITH_AES_128_CBC_SHA", tls.TLS_RSA_WITH_AES_128_CBC_SHA, false),
			Entry("unsupported cipher RSA_WITH_AES_128_GCM_SHA256", tls.TLS_RSA_WITH_AES_128_GCM_SHA256, false),
			Entry("unsupported cipher RSA_WITH_AES_256_CBC_SHA", tls.TLS_RSA_WITH_AES_256_CBC_SHA, false),
			Entry("unsupported cipher RSA_WITH_AES_256_GCM_SHA384", tls.TLS_RSA_WITH_AES_256_GCM_SHA384, false),

			Entry("supported cipher ECDHE_RSA_WITH_AES_128_GCM_SHA256", tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, true),
			Entry("supported cipher ECDHE_RSA_WITH_AES_256_GCM_SHA384", tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384, true),
		)
	})
})

var ughConfig *tls.Config

func buildTLSConfig(maxVersion, cipherSuite uint16) *tls.Config {
	if ughConfig == nil {
		ca, err := certtest.BuildCA("tlsconfig")
		Expect(err).ToNot(HaveOccurred())

		pool, err := ca.CertPool()
		Expect(err).ToNot(HaveOccurred())

		clientCrt, err := ca.BuildSignedCertificate("client")
		Expect(err).ToNot(HaveOccurred())

		clientTLSCrt, err := clientCrt.TLSCertificate()
		Expect(err).ToNot(HaveOccurred())

		ughConfig = &tls.Config{
			Certificates:       []tls.Certificate{clientTLSCrt},
			RootCAs:            pool,
			ServerName:         "",
			MaxVersion:         uint16(maxVersion),
			CipherSuites:       []uint16{cipherSuite},
			InsecureSkipVerify: true, //nolint:gosec
		}
	}

	ughConfig.MaxVersion = maxVersion
	ughConfig.CipherSuites = []uint16{cipherSuite}

	return ughConfig
}

var (
	invalidTokenResponse = //nolint:gosec
	`{
		"invalidToken": "invalid_token",
		"error_description": "Invalid token (could not decode): invalidToken"
	}`
	validTokenResponse = //nolint:gosec
	`{
		"scope": ["logs.admin"]
	}`
)

type stubLogsProvider struct {
	loggregator_v2.EgressServer

	listener net.Listener
	toSend   int
}

func newStubLogsProvider(testCerts *testservers.TestCerts) *stubLogsProvider {
	creds, err := plumbing.NewServerCredentials(
		testCerts.Cert("reverselogproxy"),
		testCerts.Key("reverselogproxy"),
		testCerts.CA(),
	)
	if err != nil {
		panic(err)
	}

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	s := &stubLogsProvider{
		listener: l,
	}

	server := grpc.NewServer(grpc.Creds(creds))
	loggregator_v2.RegisterEgressServer(server, s)

	go func() {
		err := server.Serve(s.listener)
		fmt.Println(err)
	}()

	return s
}

func (s *stubLogsProvider) Receiver(*loggregator_v2.EgressRequest, loggregator_v2.Egress_ReceiverServer) error {
	panic("not implemented")
}

func (s *stubLogsProvider) BatchedReceiver(req *loggregator_v2.EgressBatchRequest, srv loggregator_v2.Egress_BatchedReceiverServer) error {
	for i := 0; i < s.toSend; i++ {
		if isDone(srv.Context()) {
			break
		}

		Expect(srv.Send(&loggregator_v2.EnvelopeBatch{
			Batch: []*loggregator_v2.Envelope{
				{Timestamp: time.Now().UnixNano()},
			},
		})).To(Succeed())
	}

	return nil
}

func (s *stubLogsProvider) addr() string {
	return s.listener.Addr().String()
}

type testClient struct {
	mu         sync.Mutex
	_envelopes []*loggregator_v2.Envelope
	client     *http.Client
}

func newTestClient(tlsConfig *tls.Config) *testClient {
	tr := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	return &testClient{
		client: &http.Client{
			Transport: tr,
		},
	}
}

func (tc *testClient) open(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Authorization", "my-token")

	resp, err := tc.client.Do(req)
	if err != nil {
		panic(err)
	}

	if resp.StatusCode != 200 {
		return resp, nil
	}

	buf := bytes.NewBuffer(nil)
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return nil, err
		}

		switch {
		case bytes.HasPrefix(line, []byte("data:")):
			if _, err := buf.Write(line[6:]); err != nil {
				panic(fmt.Sprintf("failed to write to buffer: %s\n", err))
			}
		case bytes.Equal(line, []byte("\n")):
			var batch loggregator_v2.EnvelopeBatch
			if err := protojson.Unmarshal(buf.Bytes(), &batch); err != nil {
				panic(fmt.Sprintf("failed to unmarshal envelopes: %s", err))
			}
			buf.Reset()
			tc.mu.Lock()
			tc._envelopes = append(tc._envelopes, batch.GetBatch()...)
			tc.mu.Unlock()
		default:
			panic("unhandled SSE data")
		}
	}
}

func (tc *testClient) envelopes() []*loggregator_v2.Envelope {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	return tc._envelopes
}

func isDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

type authorizationServer struct {
	statusCode   int
	responseBody string
}

func newAuthorizationServer(code int, body, crtFile, keyFile string) *httptest.Server {
	as := &authorizationServer{
		statusCode:   code,
		responseBody: body,
	}

	return as.startTLS(crtFile, keyFile)
}

func (as *authorizationServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(as.statusCode)
	fmt.Fprintf(w, "%s", as.responseBody)
}

func (as *authorizationServer) startTLS(crt, key string) *httptest.Server {
	s := httptest.NewUnstartedServer(as)
	cert, err := tls.LoadX509KeyPair(crt, key)
	if err != nil {
		panic(fmt.Sprintf("httptest: NewTLSServer: %v", err))
	}

	s.TLS = new(tls.Config)
	s.TLS.NextProtos = []string{"http/1.1"}
	s.TLS.Certificates = []tls.Certificate{cert}
	s.Start()

	return s
}
