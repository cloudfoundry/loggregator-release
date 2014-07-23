package outputproxy_test

import (
	"trafficcontroller/outputproxy"

	"errors"
	"fmt"
	steno "github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggregatorclient"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/fakestoreadapter"
	"math/rand"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = BeforeSuite(func() {
	rand.Seed(int64(time.Now().Nanosecond()))
})

var _ = Describe("LoggregatorClientPool", func() {
	var (
		adapter  *fakestoreadapter.FakeStoreAdapter
		pool     *outputproxy.LoggregatorClientPool
		stopChan chan struct{}
		logger   *steno.Logger
	)

	BeforeEach(func() {
		adapter = fakestoreadapter.New()

		stopChan = make(chan struct{})
		logger = steno.NewLogger("TestLogger")
		pool = outputproxy.NewLoggregatorClientPool(logger)
	})

	Describe("RandomClient", func() {
		Context("with a non-empty client pool", func() {
			It("chooses a client with roughly uniform distribution", func() {
				for i := 0; i < 5; i++ {
					addr := fmt.Sprintf("127.0.0.1:%d", i)
					client := loggregatorclient.NewLoggregatorClient(addr, logger, loggregatorclient.DefaultBufferSize)
					pool.Add(addr, client)
				}

				counts := make(map[loggregatorclient.LoggregatorClient]int)
				for i := 0; i < 100000; i++ {
					pick, _ := pool.RandomClient()
					counts[pick]++
				}

				for _, count := range counts {
					Expect(count).To(BeNumerically("~", 20000, 500))
				}
			})
		})

		Context("with an empty client pool", func() {
			It("returns an error", func() {
				_, err := pool.RandomClient()
				Expect(err).To(Equal(outputproxy.ErrorEmptyClientPool))
			})
		})
	})

	Describe("RunUpdateLoop", func() {
		It("shuts down when stopChan is closed", func() {
			doneChan := make(chan struct{})

			go func() {
				pool.RunUpdateLoop(adapter, "z1", stopChan, 10*time.Millisecond)
				close(doneChan)
			}()

			close(stopChan)

			Eventually(doneChan).Should(BeClosed())
		})

		Context("when store adds a server", func() {
			var addServer = func() {
				doneChan := make(chan struct{})

				go func() {
					pool.RunUpdateLoop(adapter, "z1", stopChan, 10*time.Millisecond)
					close(doneChan)
				}()

				adapter.Create(storeadapter.StoreNode{
					Key:   "z1/loggregator_trafficcontroller_z1/0",
					Value: []byte("127.0.0.1"),
				})
			}

			Context("with 'create clients' disabled", func() {
				It("a nil client eventually appears in the pool", func() {
					defer close(stopChan)
					addServer()

					Eventually(pool.ListClients).Should(HaveLen(1))
					Expect(pool.ListClients()[0]).To(BeNil())
				})
			})

			Context("with 'create clients' enabled", func() {
				It("a non-nil client eventually appears in the pool", func() {
					defer close(stopChan)

					pool.EnableCreateClients(3456)
					addServer()

					Eventually(pool.ListClients).Should(HaveLen(1))
					Expect(pool.ListClients()[0]).ToNot(BeNil())
				})
			})

			It("adds more servers later", func() {
				defer close(stopChan)

				doneChan := make(chan struct{})

				go func() {
					pool.RunUpdateLoop(adapter, "z1", stopChan, 10*time.Millisecond)
					close(doneChan)
				}()

				adapter.Create(storeadapter.StoreNode{
					Key:   "z1/loggregator_trafficcontroller_z1/0",
					Value: []byte("127.0.0.1"),
				})

				Eventually(pool.ListClients).Should(HaveLen(1))

				adapter.Create(storeadapter.StoreNode{
					Key:   "z1/loggregator_trafficcontroller_z1/1",
					Value: []byte("127.0.0.2"),
				})

				Eventually(pool.ListClients).Should(HaveLen(2))
				Eventually(pool.ListAddresses).Should(ConsistOf("127.0.0.1", "127.0.0.2"))
			})

			It("does not duplicate known servers", func() {
				pool.Add("127.0.0.1", loggregatorclient.NewLoggregatorClient("127.0.0.1:1", logger, loggregatorclient.DefaultBufferSize))

				defer close(stopChan)

				doneChan := make(chan struct{})

				go func() {
					pool.RunUpdateLoop(adapter, "z1", stopChan, 10*time.Millisecond)
					close(doneChan)
				}()

				adapter.Create(storeadapter.StoreNode{
					Key:   "z1/loggregator_trafficcontroller_z1/0",
					Value: []byte("127.0.0.1"),
				})

				Consistently(pool.ListClients).Should(HaveLen(1))
			})
		})

		Context("when store removes a server", func() {
			It("eventually disappears from the pool", func() {
				pool.Add("127.0.0.1", loggregatorclient.NewLoggregatorClient("127.0.0.1:3456", logger, loggregatorclient.DefaultBufferSize))

				defer close(stopChan)

				doneChan := make(chan struct{})

				go func() {
					pool.RunUpdateLoop(adapter, "z1", stopChan, 10*time.Millisecond)
					close(doneChan)
				}()

				Eventually(pool.ListClients).Should(HaveLen(0))
			})
		})

		Context("when store returns an error", func() {
			BeforeEach(func() {
				adapter.ListErrInjector = fakestoreadapter.NewFakeStoreAdapterErrorInjector(".*", errors.New("list error"))
			})

			It("does not change the pool and continues to run", func() {
				defer close(stopChan)
				doneChan := make(chan struct{})

				pool.Add("127.0.0.1", loggregatorclient.NewLoggregatorClient("127.0.0.1:3456", logger, loggregatorclient.DefaultBufferSize))
				originalList := pool.ListClients()

				go func() {
					pool.RunUpdateLoop(adapter, "z1", stopChan, 10*time.Millisecond)
					close(doneChan)
				}()

				Consistently(doneChan).ShouldNot(BeClosed())
				Consistently(pool.ListClients).Should(Equal(originalList))
			})
		})
	})
})
