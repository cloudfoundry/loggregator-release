package dopplerservice_test

import (
	"errors"
	"strings"
	"sync/atomic"

	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"

	"doppler/dopplerservice"
	"doppler/dopplerservice/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

//go:generate counterfeiter -o fakes/fakestoreadapter.go ../../github.com/cloudfoundry/storeadapter StoreAdapter
var _ = Describe("Finder", func() {
	var finder dopplerservice.Finder

	Context("validation", func() {
		It("returns an error when protocol is invalid", func() {
			_, err := dopplerservice.NewFinder(nil, "bogus", nil, nil, loggertesthelper.Logger())
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Running", func() {
		var (
			order       chan string
			fakeAdapter *fakes.FakeStoreAdapter
			errChan     chan error
			stopChan    chan bool
		)

		BeforeEach(func() {
			order = make(chan string, 2)
			stopChan = make(chan bool)
			errChan = make(chan error, 1)

			fakeAdapter = &fakes.FakeStoreAdapter{}
			fakeAdapter.WatchReturns(nil, stopChan, errChan)
		})

		JustBeforeEach(func() {
			preferredCallback := func(string) bool {
				return false
			}
			var err error
			finder, err = dopplerservice.NewFinder(fakeAdapter, "udp", preferredCallback, nil, loggertesthelper.Logger())
			Expect(err).NotTo(HaveOccurred())
			finder.Start()
		})

		AfterEach(func() {
			finder.Stop()
		})

		Context("Start", func() {
			BeforeEach(func() {
				fakeAdapter.WatchStub = func(key string) (events <-chan storeadapter.WatchEvent, stop chan<- bool, errors <-chan error) {
					Expect(key).To(Equal(dopplerservice.META_ROOT))
					order <- "watch"
					return nil, stopChan, errChan
				}
				fakeAdapter.ListRecursivelyStub = func(key string) (storeadapter.StoreNode, error) {
					Expect(key).To(Equal(dopplerservice.META_ROOT))
					order <- "finder"
					return storeadapter.StoreNode{}, nil
				}
			})

			It("watches and finders nodes recursively", func() {
				Eventually(order).Should(Receive(Equal("watch")))
				Eventually(order).Should(Receive(Equal("finder")))

				By("an error occurs")

				Expect(errChan).To(BeSent(errors.New("an error")))

				Eventually(order).Should(Receive(Equal("watch")))
				Eventually(order).Should(Receive(Equal("finder")))
			})

		})

		It("Stop", func() {
			finder.Stop()
			Eventually(stopChan).Should(BeClosed())
		})
	})

	Context("with a real etcd", func() {
		var (
			storeAdapter storeadapter.StoreAdapter
			node         storeadapter.StoreNode
			updateNode   storeadapter.StoreNode

			updateCallback func(all map[string]string, preferred map[string]string)
			callbackCount  *int32

			preferredCallback func(key string) bool
			preferredCount    *int32
		)

		BeforeEach(func() {
			workPool, err := workpool.NewWorkPool(10)
			Expect(err).NotTo(HaveOccurred())
			options := &etcdstoreadapter.ETCDOptions{
				ClusterUrls: etcdRunner.NodeURLS(),
			}
			storeAdapter, err = etcdstoreadapter.New(options, workPool)
			Expect(err).NotTo(HaveOccurred())

			err = storeAdapter.Connect()
			Expect(err).NotTo(HaveOccurred())

			node = storeadapter.StoreNode{
				Key:   dopplerservice.LEGACY_ROOT + "/z1/loggregator_z1/0",
				Value: []byte("10.0.0.1"),
			}

			callbackCount = new(int32)
			count := callbackCount
			updateCallback = func(a map[string]string, p map[string]string) {
				atomic.AddInt32(count, 1)
			}

			preferredCount = new(int32)
			pCount := preferredCount
			preferredCallback = func(string) bool {
				atomic.AddInt32(pCount, 1)
				return true
			}
		})

		AfterEach(func() {
			finder.Stop()
		})

		getCallbackCount := func() int {
			return int(atomic.LoadInt32(callbackCount))
		}

		getPreferredCount := func() int {
			return int(atomic.LoadInt32(preferredCount))
		}

		testUpdateFinder := func(updatedAddress map[string]string) func() {
			return func() {
				Context("when a service exists", func() {
					JustBeforeEach(func() {
						err := storeAdapter.Create(node)
						Expect(err).NotTo(HaveOccurred())

						finder.Start()
						Eventually(finder.AllServers).ShouldNot(HaveLen(0))
						Expect(finder.PreferredServers()).NotTo(HaveLen(0))
					})

					It("updates the service", func() {
						err := storeAdapter.Update(updateNode)
						Expect(err).NotTo(HaveOccurred())

						Eventually(finder.AllServers).Should(Equal(updatedAddress))
						Expect(getCallbackCount()).To(Equal(2))
						Expect(finder.PreferredServers()).To(Equal(updatedAddress))
						Expect(getPreferredCount()).To(Equal(2))
					})
				})
			}
		}

		testFinder := func(expectedAddress map[string]string) func() {
			return func() {
				Context("when a service is created", func() {
					Context("before dopplerservice begins", func() {
						It("finds server addresses that were created before it started", func() {
							Expect(finder.AllServers()).To(HaveLen(0))
							Expect(finder.PreferredServers()).To(HaveLen(0))

							storeAdapter.Create(node)
							finder.Start()

							Eventually(finder.AllServers).Should(Equal(expectedAddress))
							Expect(getCallbackCount()).To(Equal(1))
							Expect(finder.PreferredServers()).To(Equal(expectedAddress))
							Expect(getPreferredCount()).To(Equal(1))
						})
					})

					Context("after dopplerservice begins", func() {
						It("adds servers that appear later", func() {
							finder.Start()
							Consistently(finder.AllServers, 1).Should(BeEmpty())

							storeAdapter.Create(node)

							Eventually(finder.AllServers).Should(Equal(expectedAddress))
							Expect(getCallbackCount()).To(Equal(1))
							Expect(finder.PreferredServers()).To(Equal(expectedAddress))
							Expect(getPreferredCount()).To(Equal(1))
						})
					})
				})

				Context("when services exists", func() {
					JustBeforeEach(func() {
						err := storeAdapter.Create(node)
						Expect(err).NotTo(HaveOccurred())

						finder.Start()
						Eventually(finder.AllServers).ShouldNot(HaveLen(0))
						Expect(finder.PreferredServers()).NotTo(HaveLen(0))
					})

					It("removes the service", func() {
						err := storeAdapter.Delete(node.Key)
						Expect(err).NotTo(HaveOccurred())
						Eventually(finder.AllServers).Should(BeEmpty())
						Expect(getCallbackCount()).To(Equal(2))
						Expect(finder.PreferredServers()).To(BeEmpty())
						Expect(getPreferredCount()).To(Equal(1))
					})

					It("only finds nodes for the doppler server type", func() {
						router := storeadapter.StoreNode{
							Key:   "/healthstatus/router/z1/router_z1",
							Value: []byte("10.99.99.99"),
						}
						storeAdapter.Create(router)

						Consistently(finder.AllServers).Should(Equal(expectedAddress))
						Expect(getCallbackCount()).To(Equal(1))
						Expect(finder.PreferredServers()).To(Equal(expectedAddress))
						Expect(getPreferredCount()).To(Equal(1))
					})

					Context("when ETCD is not running", func() {
						JustBeforeEach(func() {
							etcdRunner.Stop()
						})

						AfterEach(func() {
							etcdRunner.Start()
						})

						It("continues to run if the store times out", func() {
							Consistently(finder.AllServers).ShouldNot(HaveLen(0))
							Expect(finder.PreferredServers()).NotTo(HaveLen(0))
						})
					})
				})
			}
		}

		Context("LegacyFinder", func() {
			BeforeEach(func() {
				finder = dopplerservice.NewLegacyFinder(storeAdapter, 1234, preferredCallback, updateCallback, loggertesthelper.Logger())
				node = storeadapter.StoreNode{
					Key:   dopplerservice.LEGACY_ROOT + "/z1/loggregator_z1/0",
					Value: []byte("10.0.0.1"),
				}

				updateNode = node
				updateNode.Value = []byte("10.0.0.2")
			})

			Context("Basics", testFinder(map[string]string{"/z1/loggregator_z1/0": "udp://10.0.0.1:1234"}))
			Context("Update", testUpdateFinder(map[string]string{"/z1/loggregator_z1/0": "udp://10.0.0.2:1234"}))
		})

		Context("Finder", func() {
			var preferredProtocol string

			BeforeEach(func() {
				preferredProtocol = "udp"
			})

			JustBeforeEach(func() {
				var err error
				finder, err = dopplerservice.NewFinder(storeAdapter, preferredProtocol, preferredCallback, updateCallback, loggertesthelper.Logger())
				Expect(err).NotTo(HaveOccurred())
			})

			Context("with a single endpoint", func() {
				BeforeEach(func() {
					node = storeadapter.StoreNode{
						Key:   dopplerservice.META_ROOT + "/z1/loggregator_z1/0",
						Value: []byte(`{"version": 1, "endpoints":["udp://10.0.0.1:1234"]}`),
					}

					updateNode = node
					updateNode.Value = []byte(`{"version": 1, "endpoints":["udp://10.0.0.2:1234"]}`)
				})

				Context("Basics", testFinder(map[string]string{"/z1/loggregator_z1/0": "udp://10.0.0.1:1234"}))
				Context("Update", testUpdateFinder(map[string]string{"/z1/loggregator_z1/0": "udp://10.0.0.2:1234"}))
			})

			Context("with multiple endpoints", func() {
				BeforeEach(func() {
					node = storeadapter.StoreNode{
						Key:   dopplerservice.META_ROOT + "/z1/loggregator_z1/0",
						Value: []byte(`{"version": 1, "endpoints":["udp://10.0.0.1:1234", "tls://10.0.0.2:4567"]}`),
					}

					updateNode = node
					updateNode.Value = []byte(`{"version": 1, "endpoints":["udp://10.0.0.2:1235", "tls://10.0.0.3:4568"]}`)
				})

				Context("with udp preferred", func() {
					BeforeEach(func() {
						preferredProtocol = "udp"
					})

					Context("Basics", testFinder(map[string]string{"/z1/loggregator_z1/0": "udp://10.0.0.1:1234"}))
					Context("Update", testUpdateFinder(map[string]string{"/z1/loggregator_z1/0": "udp://10.0.0.2:1235"}))
				})

				Context("with tls preferred", func() {
					BeforeEach(func() {
						preferredProtocol = "tls"
					})

					Context("Basics", testFinder(map[string]string{"/z1/loggregator_z1/0": "tls://10.0.0.2:4567"}))
					Context("Update", testUpdateFinder(map[string]string{"/z1/loggregator_z1/0": "tls://10.0.0.3:4568"}))
				})
			})
		})

		Context("with a preferred filter", func() {
			BeforeEach(func() {
				preferredCallback = func(k string) bool {
					return strings.HasPrefix(k, "/z2")
				}
			})

			JustBeforeEach(func() {
				node = storeadapter.StoreNode{
					Key:   dopplerservice.META_ROOT + "/z1/loggregator_z1/0",
					Value: []byte(`{"version": 1, "endpoints":["udp://10.0.0.1:1234"]}`),
				}
				storeAdapter.Create(node)

				node2 := storeadapter.StoreNode{
					Key:   dopplerservice.META_ROOT + "/z2/loggregator_z1/0",
					Value: []byte(`{"version": 1, "endpoints":["udp://10.0.0.2:1234"]}`),
				}
				storeAdapter.Create(node2)

				var err error
				finder, err = dopplerservice.NewFinder(storeAdapter, "udp", preferredCallback, updateCallback, loggertesthelper.Logger())
				Expect(err).NotTo(HaveOccurred())
				finder.Start()
			})

			AfterEach(func() {
				finder.Stop()
			})

			It("filters", func() {
				Eventually(finder.AllServers).Should(HaveLen(2))
				Expect(finder.PreferredServers()).To(ConsistOf("udp://10.0.0.2:1234"))
			})
		})
	})
})
