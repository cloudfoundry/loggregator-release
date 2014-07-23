package main_test

import (
	main "metron"

	"errors"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
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
		pool     *main.LoggregatorClientPool
		stopChan chan struct{}
		logger   *gosteno.Logger
	)

	BeforeEach(func() {
		adapter = fakestoreadapter.New()

		stopChan = make(chan struct{})
		logger = cfcomponent.NewLogger(false, "/dev/null", "", main.Config{}.Config)
		pool = main.NewLoggregatorClientPool(logger)
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
				Expect(err).To(Equal(main.ErrorEmptyClientPool))
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
			It("eventually appears in the pool", func() {
				defer close(stopChan)

				doneChan := make(chan struct{})

				go func() {
					pool.RunUpdateLoop(adapter, "z1", stopChan, 10*time.Millisecond)
					close(doneChan)
				}()

				adapter.Create(storeadapter.StoreNode{
					Key:   "healthstatus/trafficcontroller/z1/loggregator_trafficcontroller_z1/0",
					Value: []byte("127.0.0.1:1"),
				})

				Eventually(pool.List).Should(HaveLen(1))
			})

			It("adds more servers later", func() {
				defer close(stopChan)

				doneChan := make(chan struct{})

				go func() {
					pool.RunUpdateLoop(adapter, "z1", stopChan, 10*time.Millisecond)
					close(doneChan)
				}()

				adapter.Create(storeadapter.StoreNode{
					Key:   "healthstatus/trafficcontroller/z1/loggregator_trafficcontroller_z1/0",
					Value: []byte("127.0.0.1:1"),
				})

				Eventually(pool.List).Should(HaveLen(1))

				adapter.Create(storeadapter.StoreNode{
					Key:   "healthstatus/trafficcontroller/z1/loggregator_trafficcontroller_z1/1",
					Value: []byte("127.0.0.1:2"),
				})

				Eventually(pool.List).Should(HaveLen(2))
			})

			It("does not duplicate known servers", func() {
				pool.Add("127.0.0.1:1", loggregatorclient.NewLoggregatorClient("127.0.0.1:1", logger, loggregatorclient.DefaultBufferSize))

				defer close(stopChan)

				doneChan := make(chan struct{})

				go func() {
					pool.RunUpdateLoop(adapter, "z1", stopChan, 10*time.Millisecond)
					close(doneChan)
				}()

				adapter.Create(storeadapter.StoreNode{
					Key:   "healthstatus/trafficcontroller/z1/loggregator_trafficcontroller_z1/0",
					Value: []byte("127.0.0.1:1"),
				})

				Consistently(pool.List).Should(HaveLen(1))
			})
		})

		Context("when store removes a server", func() {
			It("eventually disappears from the pool", func() {
				pool.Add("127.0.0.1:1", loggregatorclient.NewLoggregatorClient("127.0.0.1:1", logger, loggregatorclient.DefaultBufferSize))

				defer close(stopChan)

				doneChan := make(chan struct{})

				go func() {
					pool.RunUpdateLoop(adapter, "z1", stopChan, 10*time.Millisecond)
					close(doneChan)
				}()

				Eventually(pool.List).Should(HaveLen(0))
			})
		})

		Context("when store returns an error", func() {
			BeforeEach(func() {
				adapter.ListErrInjector = fakestoreadapter.NewFakeStoreAdapterErrorInjector(".*", errors.New("list error"))
			})

			It("does not change the pool and continues to run", func() {
				defer close(stopChan)
				doneChan := make(chan struct{})

				pool.Add("127.0.0.1:1", loggregatorclient.NewLoggregatorClient("127.0.0.1:1", logger, loggregatorclient.DefaultBufferSize))
				originalList := pool.List()

				go func() {
					pool.RunUpdateLoop(adapter, "z1", stopChan, 10*time.Millisecond)
					close(doneChan)
				}()

				Consistently(doneChan).ShouldNot(BeClosed())
				Consistently(pool.List).Should(Equal(originalList))
			})
		})
	})
})
