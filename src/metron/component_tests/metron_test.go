package component_test

import (
	dopplerConfig "doppler/config"
	"fmt"
	"integration_tests"
	"integration_tests/binaries"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"code.cloudfoundry.org/workpool"

	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Metron", func() {

	var (
		etcdCleanup func()
		etcdURI     string
	)

	BeforeSuite(func() {
		buildPaths, _ := binaries.Build()
		buildPaths.SetEnv()
		etcdCleanup, etcdURI = integration_tests.SetupEtcd()
	})

	AfterSuite(func() {
		etcdCleanup()
	})

	Context("when a doppler is accepting gRPC connections", func() {
		var (
			metronCleanup  func()
			dopplerCleanup func()
			metronPort     int
			dopplerPort    int
			mockDoppler    *mockDopplerIngestorServer
			eventEmitter   dropsonde.EventEmitter
		)

		BeforeEach(func() {
			dopplerPort, mockDoppler, dopplerCleanup = StartFakeGRPCDoppler()

			var metronReady func()
			metronCleanup, metronPort, metronReady = integration_tests.SetupMetron(etcdURI, dopplerPort)
			defer metronReady()

			storeAdapter := NewStoreAdapter(&dopplerConfig.Config{
				EtcdMaxConcurrentRequests: 1,
				EtcdUrls:                  []string{etcdURI},
			})

			storeAdapter.Delete("/doppler/meta/z1/doppler/0")
			node := storeadapter.StoreNode{
				Key:   "/doppler/meta/z1/doppler/0",
				Value: []byte(fmt.Sprintf(`{"version":1,"endpoints":["ws://127.0.0.1:%d"]}`, dopplerPort)),
			}
			Expect(storeAdapter.Create(node)).To(Succeed())

			udpEmitter, err := emitter.NewUdpEmitter(fmt.Sprintf("127.0.0.1:%d", metronPort))
			Expect(err).ToNot(HaveOccurred())
			eventEmitter = emitter.NewEventEmitter(udpEmitter, "some-origin")
		})

		AfterEach(func() {
			dopplerCleanup()
			metronCleanup()
		})

		It("writes to the doppler via gRPC", func() {
			envelope := &events.Envelope{
				Origin:    proto.String("some-origin"),
				EventType: events.Envelope_Error.Enum(),
				Error: &events.Error{
					Source:  proto.String("some-source"),
					Code:    proto.Int32(1),
					Message: proto.String("message"),
				},
			}

			f := func() int {
				eventEmitter.Emit(envelope)
				return len(mockDoppler.PusherCalled)
			}
			Eventually(f, 5).Should(BeNumerically(">", 0))
		})
	})

	Context("when the doppler is only accepting UDP messages", func() {
		var (
			metronCleanup  func()
			dopplerCleanup func()
			metronPort     int
			dopplerPort    int
			eventEmitter   dropsonde.EventEmitter
			dopplerConn    *net.UDPConn
		)

		BeforeEach(func() {
			addr, err := net.ResolveUDPAddr("udp4", "localhost:0")
			Expect(err).ToNot(HaveOccurred())
			dopplerConn, err = net.ListenUDP("udp4", addr)
			Expect(err).ToNot(HaveOccurred())
			dopplerPort = HomeAddrToPort(dopplerConn.LocalAddr())
			dopplerCleanup = func() {
				dopplerConn.Close()
			}

			var metronReady func()
			metronCleanup, metronPort, metronReady = integration_tests.SetupMetron(etcdURI, 0)
			defer metronReady()

			storeAdapter := NewStoreAdapter(&dopplerConfig.Config{
				EtcdMaxConcurrentRequests: 1,
				EtcdUrls:                  []string{etcdURI},
			})

			storeAdapter.Delete("/doppler/meta/z1/doppler/0")
			node := storeadapter.StoreNode{
				Key:   "/doppler/meta/z1/doppler/0",
				Value: []byte(fmt.Sprintf(`{"version":1,"endpoints":["udp://127.0.0.1:%d"]}`, dopplerPort)),
			}
			Expect(storeAdapter.Create(node)).To(Succeed())

			udpEmitter, err := emitter.NewUdpEmitter(fmt.Sprintf("127.0.0.1:%d", metronPort))
			Expect(err).ToNot(HaveOccurred())
			eventEmitter = emitter.NewEventEmitter(udpEmitter, "some-origin")
		})

		AfterEach(func() {
			dopplerCleanup()
			metronCleanup()
		})

		It("writes to the doppler via UDP", func() {
			envelope := &events.Envelope{
				Origin:    proto.String("some-origin"),
				EventType: events.Envelope_Error.Enum(),
				Error: &events.Error{
					Source:  proto.String("some-source"),
					Code:    proto.Int32(1),
					Message: proto.String("message"),
				},
			}

			c := make(chan bool, 100)
			var wg sync.WaitGroup
			wg.Add(1)
			defer wg.Wait()
			go func() {
				defer wg.Done()
				defer GinkgoRecover()

				buffer := make([]byte, 1024)
				for {
					dopplerConn.SetReadDeadline(time.Now().Add(5 * time.Second))
					_, err := dopplerConn.Read(buffer)
					Expect(err).ToNot(HaveOccurred())

					select {
					case c <- true:
					default:
						return
					}
				}
			}()

			f := func() int {
				eventEmitter.Emit(envelope)
				return len(c)
			}
			Eventually(f, 5).Should(BeNumerically(">", 0))
		})
	})
})

func NewStoreAdapter(conf *dopplerConfig.Config) storeadapter.StoreAdapter {
	workPool, err := workpool.NewWorkPool(conf.EtcdMaxConcurrentRequests)
	if err != nil {
		panic(err)
	}
	options := &etcdstoreadapter.ETCDOptions{
		ClusterUrls: conf.EtcdUrls,
	}
	if conf.EtcdRequireTLS {
		options.IsSSL = true
		options.CertFile = conf.EtcdTLSClientConfig.CertFile
		options.KeyFile = conf.EtcdTLSClientConfig.KeyFile
		options.CAFile = conf.EtcdTLSClientConfig.CAFile
	}
	etcdStoreAdapter, err := etcdstoreadapter.New(options, workPool)
	if err != nil {
		panic(err)
	}
	if err = etcdStoreAdapter.Connect(); err != nil {
		panic(err)
	}
	return etcdStoreAdapter
}

func HomeAddrToPort(addr net.Addr) int {
	port, err := strconv.Atoi(strings.Replace(addr.String(), "127.0.0.1:", "", 1))
	if err != nil {
		panic(err)
	}
	return port
}
