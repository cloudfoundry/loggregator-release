package dopplerservice_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"reflect"
	"strings"

	"code.cloudfoundry.org/loggregator/dopplerservice"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/storeadapter"
)

// Using etcd for service discovery is a deprecated code path
var _ = XDescribe("Finder", func() {
	var (
		protocols            []string
		mockStoreAdapter     *mockStoreAdapter
		legacyPort           int
		grpcPort             int
		preferredDopplerZone string

		finder *dopplerservice.Finder
	)

	BeforeEach(func() {
		preferredDopplerZone = ""
		protocols = nil
		mockStoreAdapter = newMockStoreAdapter()
		legacyPort = 1234
		grpcPort = 1235
	})

	JustBeforeEach(func() {
		finder = dopplerservice.NewFinder(mockStoreAdapter, legacyPort, grpcPort, protocols, preferredDopplerZone)
		Expect(finder).ToNot(BeNil())
	})

	Describe("Start", func() {
		JustBeforeEach(func() {
			close(mockStoreAdapter.WatchOutput.errors)
			close(mockStoreAdapter.WatchOutput.events)
			close(mockStoreAdapter.WatchOutput.stop)
			close(mockStoreAdapter.ListRecursivelyOutput.ret0)
			close(mockStoreAdapter.ListRecursivelyOutput.ret1)

			finder.Start()
		})

		It("watches and lists both roots", func() {
			Eventually(mockStoreAdapter.ListRecursivelyInput.key).Should(Receive(Equal(dopplerservice.META_ROOT)))
			Eventually(mockStoreAdapter.WatchInput.key).Should(Receive(Equal(dopplerservice.META_ROOT)))
			Eventually(mockStoreAdapter.ListRecursivelyInput.key).Should(Receive(Equal(dopplerservice.LEGACY_ROOT)))
			Eventually(mockStoreAdapter.WatchInput.key).Should(Receive(Equal(dopplerservice.LEGACY_ROOT)))
			Consistently(mockStoreAdapter.WatchCalled).Should(HaveLen(2))
		})
	})

	Describe("Next (initialization)", func() {
		var (
			metaNode   storeadapter.StoreNode
			legacyNode storeadapter.StoreNode
		)

		BeforeEach(func() {
			metaNode = storeadapter.StoreNode{}
			legacyNode = storeadapter.StoreNode{}

			close(mockStoreAdapter.WatchOutput.errors)
			close(mockStoreAdapter.WatchOutput.events)
			close(mockStoreAdapter.WatchOutput.stop)

			close(mockStoreAdapter.ListRecursivelyOutput.ret1)
		})

		JustBeforeEach(func() {
			mockStoreAdapter.ListRecursivelyOutput.ret0 <- metaNode
			mockStoreAdapter.ListRecursivelyOutput.ret0 <- legacyNode
			finder.Start()
		})

		Context("no available dopplers", func() {
			var (
				metaEvents chan storeadapter.WatchEvent
				done       chan struct{}
			)

			BeforeEach(func() {
				done = make(chan struct{})
				metaEvents = make(chan storeadapter.WatchEvent)
				mockStoreAdapter.WatchOutput.events = make(chan (<-chan storeadapter.WatchEvent), 1)
				mockStoreAdapter.WatchOutput.events <- metaEvents
				close(mockStoreAdapter.WatchOutput.events)
			})

			AfterEach(func() {
				node := makeMetaNode("z1/doppler_z1/0", []string{"tls://1.2.3.4:567"})
				metaEvents <- storeadapter.WatchEvent{
					Type: storeadapter.CreateEvent,
					Node: &node,
				}
				Eventually(done).Should(BeClosed())
			})

			It("doesn't send an event", func() {
				go func() {
					finder.Next()
					close(done)
				}()
				Consistently(done).ShouldNot(BeClosed())
			})
		})

		Context("invalid protocol", func() {
			BeforeEach(func() {
				protocols = []string{"udp", "tls"}
				metaNode, legacyNode = etcdNodes(map[string][]string{
					"z1/doppler_z1/0": {"udp://1.2.3.4:567"},
					"z1/doppler_z1/1": {"tls://9.8.7.6:555"},
					"z1/doppler_z1/2": {"xyz://9.8.7.6:543", "tca://9.8.7.6:555"},
				}, nil)
			})

			It("ignores invalid protocol", func() {
				expected := dopplerservice.Event{
					UDPDopplers: []string{"1.2.3.4:567"},
					TLSDopplers: []string{"9.8.7.6:555"},
				}
				Expect(finder.Next()).To(Equal(expected))
			})
		})

		Context("with invalid JSON", func() {
			BeforeEach(func() {
				protocols = []string{"udp"}
				metaNode, legacyNode = etcdNodes(map[string][]string{
					"z1/doppler_z1/0": {"udp://1.2.3.4:567"},
				}, nil)
				metaNode.ChildNodes = append(metaNode.ChildNodes, storeadapter.StoreNode{
					Key:   path.Join(dopplerservice.META_ROOT, "z1/doppler_z1/0"),
					Value: []byte(`gobbledegook`),
				})
			})

			It("ignores the doppler", func() {
				expected := dopplerservice.Event{
					UDPDopplers: []string{"1.2.3.4:567"},
				}
				Expect(finder.Next()).To(Equal(expected))
			})
		})

		Context("with data in the meta node", func() {
			BeforeEach(func() {
				protocols = []string{"tls", "udp", "ws"}
				metaNode, legacyNode = etcdNodes(map[string][]string{
					"z1/doppler_z1/0": {"udp://1.2.3.4:567"},
					"z1/doppler_z1/1": {"udp://9.8.7.6:543", "tls://9.8.7.6:555"},
					"z1/doppler_z1/2": {},
					"z2/doppler_z2/0": {"ws://2.3.4.5:789"},
				}, nil)
			})

			It("returns the endpoint of each doppler", func() {
				event := finder.Next()
				Expect(event.UDPDopplers).To(ConsistOf([]string{"1.2.3.4:567", "9.8.7.6:543"}))
				Expect(event.TLSDopplers).To(Equal([]string{"9.8.7.6:555"}))
				Expect(event.GRPCDopplers).To(Equal([]string{fmt.Sprintf("2.3.4.5:%d", grpcPort)}))
			})
		})

		Context("with data in the legacy node and udp in protocols", func() {
			BeforeEach(func() {
				protocols = []string{"udp"}
				metaNode, legacyNode = etcdNodes(nil, map[string]string{
					"z1/doppler_z1/0": "1.2.3.4",
					"z1/doppler_z1/1": "5.6.7.8",
				})
			})

			It("returns the endpoint of each doppler, with the protocol and port", func() {
				event := finder.Next()
				firstURL := fmt.Sprintf("1.2.3.4:%d", legacyPort)
				secondURL := fmt.Sprintf("5.6.7.8:%d", legacyPort)
				Expect(event.UDPDopplers).To(ConsistOf(firstURL, secondURL))
				Expect(event.TLSDopplers).To(BeEmpty())
			})
		})

		Context("with data in both nodes", func() {
			BeforeEach(func() {
				protocols = []string{"udp", "tls"}
				metaNode, legacyNode = etcdNodes(map[string][]string{
					"z1/doppler_z1/0": {"udp://1.2.3.4:567"},
					"z1/doppler_z1/1": {"udp://9.8.7.6:543", "tls://9.8.7.6:555"},
					"z1/doppler_z1/2": {"tls://11.21.31.41:1234"},
					"z1/doppler_z1/4": {"tls://31.32.33.34:1234"},
				}, map[string]string{
					"z1/doppler_z1/2": "11.21.31.41",
					"z1/doppler_z1/3": "21.22.23.24",
				})
			})

			It("merges the nodes", func() {
				expectedUDP := []string{
					"1.2.3.4:567",
					"9.8.7.6:543",
					fmt.Sprintf("11.21.31.41:%d", legacyPort),
					fmt.Sprintf("21.22.23.24:%d", legacyPort),
				}
				expectedTLS := []string{
					"31.32.33.34:1234",
					"9.8.7.6:555",
				}
				event := finder.Next()
				Expect(event.UDPDopplers).To(ConsistOf(expectedUDP))
				Expect(event.TLSDopplers).To(ConsistOf(expectedTLS))
			})
		})

		Context("with preferred dopplers in the pool", func() {
			BeforeEach(func() {
				protocols = []string{"udp", "tls"}
				preferredDopplerZone = "z2"
				metaNode, legacyNode = etcdNodes(map[string][]string{
					"z1/doppler_z1/0": {"udp://1.2.3.4:567"},
					"z2/doppler_z3/1": {"tls://11.21.31.41:1234"},
					"z2/doppler_z3/2": {"tls://11.21.31.42:1234"},
				}, map[string]string{
					"z1/doppler_z2/0": "11.21.31.41",
					"z2/doppler_z3/2": "21.22.23.24",
				})
			})

			It("skips non-preferred doppler instances", func() {
				expectedUDP := []string{fmt.Sprintf("21.22.23.24:%d", legacyPort)}
				expectedTLS := []string{"11.21.31.41:1234"}

				event := finder.Next()
				Expect(event.UDPDopplers).To(Equal(expectedUDP))
				Expect(event.TLSDopplers).To(Equal(expectedTLS))
			})
		})

		Context("without any preferred dopplers in the pool", func() {
			BeforeEach(func() {
				preferredDopplerZone = "z4"
				protocols = []string{"udp", "tls"}
				metaNode, legacyNode = etcdNodes(map[string][]string{
					"z1/doppler_z1/0": {"udp://1.2.3.4:567"},
					"z3/doppler_z3/2": {"tls://11.21.31.41:1234"},
				}, map[string]string{
					"z2/doppler_z2/2": "11.21.31.41",
					"z3/doppler_z3/3": "21.22.23.24",
				})
			})

			It("returns all available doppler instances", func() {
				expectedUDP := []string{
					"1.2.3.4:567",
					fmt.Sprintf("11.21.31.41:%d", legacyPort),
					fmt.Sprintf("21.22.23.24:%d", legacyPort),
				}
				expectedTLS := []string{"11.21.31.41:1234"}

				event := finder.Next()
				Expect(event.UDPDopplers).To(HaveLen(3))
				Expect(event.UDPDopplers).To(ConsistOf(expectedUDP[0], expectedUDP[1], expectedUDP[2]))
				Expect(event.TLSDopplers).To(Equal(expectedTLS))
			})
		})
	})

	Describe("Next (async)", func() {
		var (
			metaNode   storeadapter.StoreNode
			legacyNode storeadapter.StoreNode

			metaErrs   chan error
			metaEvents chan storeadapter.WatchEvent
			metaStop   chan bool

			legacyErrs   chan error
			legacyEvents chan storeadapter.WatchEvent
			legacyStop   chan bool
		)

		BeforeEach(func() {
			metaNode = storeadapter.StoreNode{}
			legacyNode = storeadapter.StoreNode{}

			metaErrs = make(chan error, 100)
			metaEvents = make(chan storeadapter.WatchEvent, 100)
			metaStop = make(chan bool)

			legacyErrs = make(chan error, 100)
			legacyEvents = make(chan storeadapter.WatchEvent, 100)
			legacyStop = make(chan bool)

			mockStoreAdapter.WatchOutput.errors <- metaErrs
			mockStoreAdapter.WatchOutput.events <- metaEvents
			mockStoreAdapter.WatchOutput.stop <- metaStop

			mockStoreAdapter.WatchOutput.errors <- legacyErrs
			mockStoreAdapter.WatchOutput.events <- legacyEvents
			mockStoreAdapter.WatchOutput.stop <- legacyStop

			close(mockStoreAdapter.ListRecursivelyOutput.ret1)
		})

		JustBeforeEach(func() {
			mockStoreAdapter.ListRecursivelyOutput.ret0 <- metaNode
			mockStoreAdapter.ListRecursivelyOutput.ret0 <- legacyNode
			finder.Start()
		})

		It("reconnects when the meta watch errors receives an error", func() {
			// reset the watch input because we don't care about any previous watch calls
			mockStoreAdapter.WatchInput.key = make(chan string, 100)

			metaErrs <- errors.New("disconnected")
			Eventually(mockStoreAdapter.WatchInput.key, 2).Should(Receive(Equal(dopplerservice.META_ROOT)))
			Eventually(mockStoreAdapter.ListRecursivelyInput.key).Should(Receive(Equal(dopplerservice.META_ROOT)))
		})

		It("reconnects when the legacy watch errors receives an error", func() {
			// reset the watch input because we don't care about any previous watch calls
			mockStoreAdapter.WatchInput.key = make(chan string, 100)

			legacyErrs <- errors.New("disconnected")
			Eventually(mockStoreAdapter.WatchInput.key, 2).Should(Receive(Equal(dopplerservice.LEGACY_ROOT)))
			Eventually(mockStoreAdapter.ListRecursivelyInput.key).Should(Receive(Equal(dopplerservice.META_ROOT)))
		})

		Describe("GRPC URLs", func() {
			BeforeEach(func() {
				protocols = []string{"ws"}
			})

			Context("when the node is created", func() {
				It("returns websocket URLs in finder event", func() {
					node := makeMetaNode("z1/doppler_z1/0", []string{"ws://1.2.3.4:567"})
					metaEvents <- storeadapter.WatchEvent{
						Type: storeadapter.CreateEvent,
						Node: &node,
					}

					event := finder.Next()
					Expect(event.GRPCDopplers).To(Equal([]string{fmt.Sprintf("1.2.3.4:%d", grpcPort)}))
				})
			})

			Context("when the node value is updated", func() {
				It("returns the updated URL", func() {
					node := makeMetaNode("z1/doppler_z1/0", []string{"ws://1.2.3.4:567"})
					updatedNode := makeMetaNode("z1/doppler_z1/0", []string{"ws://1.2.3.7:678"})
					metaEvents <- storeadapter.WatchEvent{
						Type:     storeadapter.UpdateEvent,
						Node:     &updatedNode,
						PrevNode: &node,
					}

					event := finder.Next()
					Expect(event.GRPCDopplers).To(Equal([]string{fmt.Sprintf("1.2.3.7:%d", grpcPort)}))
				})
			})

			Context("when the node is removed", func() {
				It("removes the entry", func() {
					node := makeMetaNode("z1/doppler_z1/0", []string{"ws://1.2.3.4:567"})

					metaEvents <- storeadapter.WatchEvent{
						Type: storeadapter.CreateEvent,
						Node: &node,
					}

					event := finder.Next()
					Expect(event.GRPCDopplers).ToNot(BeEmpty())

					metaEvents <- storeadapter.WatchEvent{
						Type:     storeadapter.DeleteEvent,
						PrevNode: &node,
					}

					event = finder.Next()
					Expect(event.GRPCDopplers).To(BeEmpty())
				})
			})
		})

		Context("meta endpoints", func() {
			BeforeEach(func() {
				protocols = []string{"tls"}
			})

			Context("when a new node is created", func() {
				JustBeforeEach(func() {
					node := makeMetaNode("z1/doppler_z1/0", []string{"tls://1.2.3.4:567"})
					metaEvents <- storeadapter.WatchEvent{
						Type: storeadapter.CreateEvent,
						Node: &node,
					}
				})

				It("returns the endpoints, including the new meta endpoint", func() {
					event := finder.Next()
					Expect(event.TLSDopplers).To(Equal([]string{"1.2.3.4:567"}))
					Expect(event.UDPDopplers).To(BeEmpty())
					Expect(event.TCPDopplers).To(BeEmpty())
					Expect(event.GRPCDopplers).To(BeEmpty())
				})
			})

			Context("when a node is updated", func() {
				BeforeEach(func() {
					metaNode = makeMetaNode("z1/doppler_z1/0", []string{
						"udp://1.2.3.4:567",
						"tcp://1.2.3.4:789",
					})
				})

				JustBeforeEach(func() {
					// Ignore the startup event
					_ = finder.Next()

					updateNode := makeMetaNode("z1/doppler_z1/0", []string{
						"tls://1.2.3.4:555",
						"udp://1.2.3.4:567",
					})
					metaEvents <- storeadapter.WatchEvent{
						Type:     storeadapter.UpdateEvent,
						Node:     &updateNode,
						PrevNode: &metaNode,
					}
				})

				It("returns the updated endpoints", func() {
					event := finder.Next()
					Expect(event.TLSDopplers).To(Equal([]string{"1.2.3.4:555"}))
					Expect(event.UDPDopplers).To(Equal([]string{"1.2.3.4:567"}))
					Expect(event.TCPDopplers).To(BeEmpty())
					Expect(event.GRPCDopplers).To(BeEmpty())
				})
			})

			Context("when a node's value order is changed", func() {
				var done chan struct{}

				BeforeEach(func() {
					metaNode = makeMetaNode("z1/doppler_z1/0", []string{"udp://1.2.3.4:567", "tcp://1.2.3.4:578"})
				})

				JustBeforeEach(func() {
					// Ignore the startup event
					_ = finder.Next()

					updateNode := makeMetaNode("z1/doppler_z1/0", []string{"tcp://1.2.3.4:578", "udp://1.2.3.4:567"})

					metaEvents <- storeadapter.WatchEvent{
						Type:     storeadapter.UpdateEvent,
						Node:     &updateNode,
						PrevNode: &metaNode,
					}

					done = make(chan struct{})
				})

				AfterEach(func() {
					node := makeMetaNode("z1/doppler_z1/0", []string{"tls://1.2.3.4:567"})
					metaEvents <- storeadapter.WatchEvent{
						Type: storeadapter.CreateEvent,
						Node: &node,
					}
					Eventually(done).Should(BeClosed())
				})

				It("doesn't send an event", func() {
					go func() {
						finder.Next()
						close(done)
					}()
					Consistently(done).ShouldNot(BeClosed())
				})
			})

			Context("when a node's TTL is updated without a value change", func() {
				var done chan struct{}

				BeforeEach(func() {
					metaNode = makeMetaNode("z1/doppler_z1/0", []string{"udp://1.2.3.4:567"})
					metaNode.TTL = 30
				})

				JustBeforeEach(func() {
					// Ignore the startup event
					_ = finder.Next()

					// Pretend it's been 20 seconds
					updateNode := metaNode
					metaNode.TTL = 10
					metaEvents <- storeadapter.WatchEvent{
						Type:     storeadapter.UpdateEvent,
						Node:     &updateNode,
						PrevNode: &metaNode,
					}

					done = make(chan struct{})
				})

				AfterEach(func() {
					node := makeMetaNode("z1/doppler_z1/0", []string{"tls://1.2.3.4:567"})
					metaEvents <- storeadapter.WatchEvent{
						Type: storeadapter.CreateEvent,
						Node: &node,
					}
					Eventually(done).Should(BeClosed())
				})

				It("doesn't send an event", func() {
					go func() {
						finder.Next()
						close(done)
					}()
					Consistently(done).ShouldNot(BeClosed())
				})
			})

			Context("when a node is deleted", func() {
				BeforeEach(func() {
					metaNode = makeMetaNode("z1/doppler_z1/0", []string{"tls://1.2.3.4:567"})
				})

				JustBeforeEach(func() {
					// Ignore the startup event
					_ = finder.Next()

					metaEvents <- storeadapter.WatchEvent{
						Type:     storeadapter.DeleteEvent,
						PrevNode: &metaNode,
					}
				})

				It("removes the entry from metaEndpoints", func() {
					event := finder.Next()
					Expect(event.TLSDopplers).To(BeEmpty())
				})
			})

			Context("when a node is expired", func() {
				BeforeEach(func() {
					metaNode = makeMetaNode("z1/doppler_z1/0", []string{"tls://1.2.3.4:567"})
				})

				JustBeforeEach(func() {
					// Ignore the startup event
					_ = finder.Next()

					metaEvents <- storeadapter.WatchEvent{
						Type:     storeadapter.ExpireEvent,
						PrevNode: &metaNode,
					}
				})

				It("removes the entry from metaEndpoints", func() {
					event := finder.Next()
					Expect(event.TLSDopplers).To(BeEmpty())
				})
			})

			Context("when invalid event is received", func() {
				JustBeforeEach(func() {
					metaEvents <- storeadapter.WatchEvent{
						Type: storeadapter.InvalidEvent,
					}
				})

				It("does not send an event", func() {
					done := make(chan struct{})
					go func() {
						finder.Next()
						close(done)
					}()
					Consistently(done).ShouldNot(BeClosed())

					// Get the goroutine to end
					node := makeMetaNode("z1/doppler_z1/0", []string{"tls://1.2.3.4:567"})
					metaEvents <- storeadapter.WatchEvent{
						Type:     storeadapter.CreateEvent,
						Node:     &node,
						PrevNode: nil,
					}
					Eventually(done).Should(BeClosed())
				})
			})
		})

		Context("legacy endpoints", func() {
			BeforeEach(func() {
				protocols = []string{"udp"}
			})

			Context("when a new node is created", func() {
				JustBeforeEach(func() {
					node := storeadapter.StoreNode{
						Key:   path.Join(dopplerservice.LEGACY_ROOT, "z1/doppler_z1/0"),
						Value: []byte("1.2.3.4"),
					}
					legacyEvents <- storeadapter.WatchEvent{
						Type: storeadapter.CreateEvent,
						Node: &node,
					}
				})

				It("returns the endpoints, including the new legacy endpoint", func() {
					event := finder.Next()
					Expect(event.TLSDopplers).To(BeEmpty())
					Expect(event.UDPDopplers).To(Equal([]string{fmt.Sprintf("1.2.3.4:%d", legacyPort)}))
				})
			})

			Context("when a node is updated", func() {
				BeforeEach(func() {
					legacyNode = storeadapter.StoreNode{
						Key:   path.Join(dopplerservice.LEGACY_ROOT, "z1/doppler_z1/0"),
						Value: []byte("1.2.3.4"),
					}
				})

				JustBeforeEach(func() {
					// Ignore the startup event
					_ = finder.Next()

					updateNode := storeadapter.StoreNode{
						Key:   legacyNode.Key,
						Value: []byte("5.6.7.8"),
					}
					legacyEvents <- storeadapter.WatchEvent{
						Type:     storeadapter.UpdateEvent,
						Node:     &updateNode,
						PrevNode: &legacyNode,
					}
				})

				It("returns the updated endpoint", func() {
					event := finder.Next()
					Expect(event.TLSDopplers).To(BeEmpty())
					Expect(event.UDPDopplers).To(Equal([]string{fmt.Sprintf("5.6.7.8:%d", legacyPort)}))
				})
			})

			Context("when a node's TTL is updated", func() {
				var done chan struct{}

				BeforeEach(func() {
					done = make(chan struct{})

					legacyNode = storeadapter.StoreNode{
						Key:   path.Join(dopplerservice.LEGACY_ROOT, "z1/doppler_z1/0"),
						Value: []byte("1.2.3.4"),
						TTL:   30,
					}
				})

				JustBeforeEach(func() {
					// Ignore the startup event
					_ = finder.Next()

					updateNode := legacyNode
					legacyNode.TTL = 10
					legacyEvents <- storeadapter.WatchEvent{
						Type:     storeadapter.UpdateEvent,
						Node:     &updateNode,
						PrevNode: &legacyNode,
					}
				})

				AfterEach(func() {
					node := storeadapter.StoreNode{
						Key:   path.Join(dopplerservice.LEGACY_ROOT, "z1/doppler_z1/1"),
						Value: []byte("1.2.3.5"),
						TTL:   30,
					}
					legacyEvents <- storeadapter.WatchEvent{
						Type: storeadapter.CreateEvent,
						Node: &node,
					}
					Eventually(done).Should(BeClosed())
				})

				It("doesn't send an event", func() {
					go func() {
						finder.Next()
						close(done)
					}()
					Consistently(done).ShouldNot(BeClosed())
				})
			})

			Context("when a node is deleted", func() {
				BeforeEach(func() {
					legacyNode = storeadapter.StoreNode{
						Key:   path.Join(dopplerservice.LEGACY_ROOT, "z1/doppler_z1/0"),
						Value: []byte("1.2.3.4"),
					}
				})

				JustBeforeEach(func() {
					// Ignore the startup event
					_ = finder.Next()

					legacyEvents <- storeadapter.WatchEvent{
						Type:     storeadapter.DeleteEvent,
						PrevNode: &legacyNode,
					}
				})

				It("removes the entry from legacyEndpoints", func() {
					event := finder.Next()
					Expect(event.UDPDopplers).To(BeEmpty())
				})
			})

			Context("when a node is expired", func() {
				BeforeEach(func() {
					legacyNode = storeadapter.StoreNode{
						Key:   path.Join(dopplerservice.LEGACY_ROOT, "z1/doppler_z1/0"),
						Value: []byte("1.2.3.4"),
					}
				})

				JustBeforeEach(func() {
					// Ignore the startup event
					_ = finder.Next()

					legacyEvents <- storeadapter.WatchEvent{
						Type:     storeadapter.ExpireEvent,
						PrevNode: &legacyNode,
					}
				})

				It("removes the entry from legacyEndpoints", func() {
					event := finder.Next()
					Expect(event.UDPDopplers).To(BeEmpty())
				})
			})

			Context("when invalid event is received", func() {
				JustBeforeEach(func() {
					legacyEvents <- storeadapter.WatchEvent{
						Type: storeadapter.InvalidEvent,
					}
				})

				It("does not send an event", func() {
					done := make(chan struct{})
					go func() {
						finder.Next()
						close(done)
					}()
					Consistently(done).ShouldNot(BeClosed())

					// Get the goroutine to end
					node := makeMetaNode("z1/doppler_z1/0", []string{"tls://1.2.3.4:567"})
					metaEvents <- storeadapter.WatchEvent{
						Type:     storeadapter.CreateEvent,
						Node:     &node,
						PrevNode: nil,
					}
					Eventually(done).Should(BeClosed())
				})
			})
		})

		Context("with a preferred zone", func() {
			BeforeEach(func() {
				metaNode, legacyNode = storeadapter.StoreNode{}, storeadapter.StoreNode{}
				protocols = []string{"tcp", "tls", "udp"}
				preferredDopplerZone = "z2"
			})

			DescribeTable("preferred zones override other zones", func(protocol string) {
				By("falling back to z1")
				z1Node := makeMetaNode("z1/doppler_z1/0", []string{protocol + "://1.2.3.4:567"})
				metaEvents <- storeadapter.WatchEvent{
					Node: &z1Node,
					Type: storeadapter.CreateEvent,
				}
				event := finder.Next()
				Expect(protocolField(event, protocol)).To(ConsistOf("1.2.3.4:567"))

				By("overriding z1 with z2")
				z2Node := makeMetaNode("z2/doppler_z2/0", []string{protocol + "://11.21.31.41:1234"})
				metaEvents <- storeadapter.WatchEvent{
					Node: &z2Node,
					Type: storeadapter.CreateEvent,
				}
				event = finder.Next()
				Expect(protocolField(event, protocol)).To(ConsistOf("11.21.31.41:1234"))

				By("ignoring updates to z3 when z2 still has dopplers")
				z3Node := makeMetaNode("z3/doppler_z3/0", []string{protocol + "://4.3.2.1:4321"})
				metaEvents <- storeadapter.WatchEvent{
					Node: &z3Node,
					Type: storeadapter.CreateEvent,
				}
				event = finder.Next()
				Expect(protocolField(event, protocol)).To(ConsistOf("11.21.31.41:1234"))
			},
				Entry("udp", "udp"),
				Entry("tcp", "tcp"),
				Entry("tls", "tls"),
			)
		})
	})

	Context("with TCP preferred over UDP", func() {
		var (
			metaNode, legacyNode storeadapter.StoreNode
			metaServers          map[string][]string
			legacyServers        map[string]string
		)

		BeforeEach(func() {
			legacyPort = 9999
			protocols = []string{"tcp", "udp"}

			close(mockStoreAdapter.WatchOutput.errors)
			close(mockStoreAdapter.WatchOutput.events)
			close(mockStoreAdapter.WatchOutput.stop)
			close(mockStoreAdapter.ListRecursivelyOutput.ret1)
		})

		JustBeforeEach(func() {
			metaNode, legacyNode = etcdNodes(metaServers, legacyServers)
			mockStoreAdapter.ListRecursivelyOutput.ret0 <- metaNode
			mockStoreAdapter.ListRecursivelyOutput.ret0 <- legacyNode
			finder.Start()
		})

		Context("with dopplers advertising on legacy root only", func() {
			BeforeEach(func() {
				metaServers = map[string][]string{}
				legacyServers = map[string]string{
					"z1/doppler_z1/2": "11.21.31.41",
					"z1/doppler_z1/3": "21.22.23.24",
				}
			})

			It("returns UDP/TLS dopplers from legacy endpoint", func() {
				event := finder.Next()
				Expect(event.UDPDopplers).To(ConsistOf(
					fmt.Sprintf("%s:%d", legacyServers["z1/doppler_z1/2"], legacyPort),
					fmt.Sprintf("%s:%d", legacyServers["z1/doppler_z1/3"], legacyPort),
				))
				Expect(event.TLSDopplers).To(BeEmpty())
				Expect(event.TCPDopplers).To(BeEmpty())
			})
		})

		Context("with dopplers advertising UDP on meta and legacy root", func() {
			BeforeEach(func() {
				metaServers = map[string][]string{
					"z1/doppler_z1/0": {"udp://9.8.7.6:3457"},
				}
				legacyServers = map[string]string{
					"z1/doppler_z1/0": "9.8.7.6",
					"z1/doppler_z1/1": "21.22.23.24",
				}
			})

			It("returns udp dopplers from meta endpoint", func() {
				event := finder.Next()
				Expect(event.UDPDopplers).To(ConsistOf(
					"9.8.7.6:3457",
					fmt.Sprintf("%s:%d", legacyServers["z1/doppler_z1/1"], legacyPort),
				))
				Expect(event.TLSDopplers).To(BeEmpty())
				Expect(event.TCPDopplers).To(BeEmpty())
			})
		})

		Context("with dopplers advertising UDP, TCP, and TLS on meta and legacy root", func() {
			BeforeEach(func() {
				metaServers = map[string][]string{
					"z1/doppler_z1/0": {
						"udp://9.8.7.6:3457",
						"tcp://9.8.7.6:3459",
						"tls://9.8.7.6:3458",
					},
					"z1/doppler_z1/2": {
						"udp://9.8.7.7:3457",
						"tls://9.8.7.7:3458",
					},
				}
				legacyServers = map[string]string{
					"z1/doppler_z1/0": "9.8.7.6",
					"z1/doppler_z1/1": "21.22.23.24",
				}
			})

			It("returns dopplers from meta endpoint", func() {
				event := finder.Next()
				Expect(event.UDPDopplers).To(ConsistOf(
					"9.8.7.7:3457",
					"9.8.7.6:3457",
					fmt.Sprintf("%s:%d", legacyServers["z1/doppler_z1/1"], legacyPort),
				))
				Expect(event.TLSDopplers).To(ConsistOf([]string{"9.8.7.7:3458", "9.8.7.6:3458"}))
				Expect(event.TCPDopplers).To(ConsistOf("9.8.7.6:3459"))
			})
		})

		Context("with dopplers advertising UDP and an unsupported protocol", func() {
			BeforeEach(func() {
				metaServers = map[string][]string{
					"z1/doppler_z1/0": {
						"https://1.2.3.4/foo",
						"udp://1.2.3.4:5678",
					},
				}
				legacyServers = map[string]string{}
			})

			It("returns the supported address for that doppler", func() {
				event := finder.Next()
				Expect(event.UDPDopplers).To(ConsistOf("1.2.3.4:5678"))
				Expect(event.TCPDopplers).To(BeEmpty())
				Expect(event.TLSDopplers).To(BeEmpty())
			})
		})
	})

	Context("with TLS Preferred over UDP", func() {
		var (
			metaNode, legacyNode storeadapter.StoreNode
			metaServers          map[string][]string
			legacyServers        map[string]string
		)

		BeforeEach(func() {
			legacyPort = 9999
			protocols = []string{"tls", "udp"}

			close(mockStoreAdapter.WatchOutput.errors)
			close(mockStoreAdapter.WatchOutput.events)
			close(mockStoreAdapter.WatchOutput.stop)
			close(mockStoreAdapter.ListRecursivelyOutput.ret1)
		})

		JustBeforeEach(func() {
			metaNode, legacyNode = etcdNodes(metaServers, legacyServers)
			mockStoreAdapter.ListRecursivelyOutput.ret0 <- metaNode
			mockStoreAdapter.ListRecursivelyOutput.ret0 <- legacyNode
			finder.Start()
		})

		Context("with dopplers advertising on legacy root only", func() {
			BeforeEach(func() {
				metaServers = map[string][]string{}
				legacyServers = map[string]string{
					"z1/doppler_z1/2": "11.21.31.41",
					"z1/doppler_z1/3": "21.22.23.24",
				}
			})

			It("returns udp dopplers from legacy endpoint", func() {
				event := finder.Next()
				Expect(event.UDPDopplers).To(ConsistOf(
					fmt.Sprintf("11.21.31.41:%d", legacyPort),
					fmt.Sprintf("21.22.23.24:%d", legacyPort),
				))
				Expect(event.TLSDopplers).To(BeEmpty())
				Expect(event.TCPDopplers).To(BeEmpty())
			})
		})

		Context("with dopplers advertising UDP on meta and legacy root", func() {
			BeforeEach(func() {
				metaServers = map[string][]string{
					"z1/doppler_z1/0": {"udp://9.8.7.6:3457"},
				}
				legacyServers = map[string]string{
					"z1/doppler_z1/0": "9.8.7.6",
					"z1/doppler_z1/1": "21.22.23.24",
				}
			})

			It("returns udp dopplers from meta endpoint", func() {
				event := finder.Next()
				Expect(event.UDPDopplers).To(ConsistOf(
					"9.8.7.6:3457",
					fmt.Sprintf("21.22.23.24:%d", legacyPort),
				))
				Expect(event.TLSDopplers).To(BeEmpty())
				Expect(event.TCPDopplers).To(BeEmpty())
			})
		})

		Context("with dopplers advertising TLS, TCP, and UDP on meta and legacy root", func() {
			BeforeEach(func() {
				metaServers = map[string][]string{
					"z1/doppler_z1/0": {
						"udp://9.8.7.6:3457",
						"tcp://9.8.7.6:3459",
						"tls://9.8.7.6:3458",
					},
				}
				legacyServers = map[string]string{
					"z1/doppler_z1/0": "9.8.7.6",
					"z1/doppler_z1/1": "21.22.23.24",
				}
			})

			It("returns dopplers from meta endpoint", func() {
				event := finder.Next()
				Expect(event.UDPDopplers).To(ConsistOf(
					"9.8.7.6:3457",
					fmt.Sprintf("21.22.23.24:%d", legacyPort),
				))
				Expect(event.TCPDopplers).To(ConsistOf("9.8.7.6:3459"))
				Expect(event.TLSDopplers).To(ConsistOf("9.8.7.6:3458"))
			})
		})

		Context("with dopplers advertising UDP and unsupported protocols", func() {
			BeforeEach(func() {
				metaServers = map[string][]string{
					"z1/doppler_z1/0": {
						"https://1.2.3.4/foo",
						"udp://1.2.3.4:5678",
						"wss://1.2.3.4/bar",
					},
				}
				legacyServers = map[string]string{}
			})

			It("returns the supported address for that doppler", func() {
				event := finder.Next()
				Expect(event.UDPDopplers).To(ConsistOf("1.2.3.4:5678"))
				Expect(event.TCPDopplers).To(BeEmpty())
				Expect(event.TLSDopplers).To(BeEmpty())
			})
		})
	})

	Context("with multiple protocols", func() {
		BeforeEach(func() {
			legacyPort = 9999
			protocols = []string{"tcp", "udp", "tls"}

			close(mockStoreAdapter.WatchOutput.errors)
			close(mockStoreAdapter.WatchOutput.events)
			close(mockStoreAdapter.WatchOutput.stop)
			close(mockStoreAdapter.ListRecursivelyOutput.ret1)
		})

		JustBeforeEach(func() {
			metaServers := map[string][]string{
				"z1/doppler_z1/0": {
					"udp://9.8.7.6:3457",
					"tls://9.8.7.6:3458",
					"tcp://9.8.7.6:3459",
				},
				"z1/doppler_z1/1": {
					"udp://9.8.7.7:3457",
					"tls://9.8.7.7:3458",
				},
			}
			metaNode, legacyNode := etcdNodes(metaServers, nil)
			mockStoreAdapter.ListRecursivelyOutput.ret0 <- metaNode
			mockStoreAdapter.ListRecursivelyOutput.ret0 <- legacyNode
			finder.Start()
		})

		It("returns only the addrs for the highest priority dopplers", func() {
			event := finder.Next()
			Expect(event.TLSDopplers).To(HaveLen(2))
			Expect(event.UDPDopplers).To(HaveLen(2))
			Expect(event.UDPDopplers).To(ConsistOf("9.8.7.7:3457", "9.8.7.6:3457"))
			Expect(event.TCPDopplers).To(HaveLen(1))
			Expect(event.TCPDopplers).To(ConsistOf("9.8.7.6:3459"))
		})
	})

	Context("UDP Preferred Protocol on Metron", func() {
		var (
			metaServers   map[string][]string
			legacyServers map[string]string
		)

		BeforeEach(func() {
			legacyPort = 9999
			protocols = []string{"udp"}

			close(mockStoreAdapter.WatchOutput.errors)
			close(mockStoreAdapter.WatchOutput.events)
			close(mockStoreAdapter.WatchOutput.stop)
			close(mockStoreAdapter.ListRecursivelyOutput.ret1)
		})

		JustBeforeEach(func() {
			metaNode, legacyNode := etcdNodes(metaServers, legacyServers)
			mockStoreAdapter.ListRecursivelyOutput.ret0 <- metaNode
			mockStoreAdapter.ListRecursivelyOutput.ret0 <- legacyNode
			finder.Start()
		})

		Context("only legacy endpoints available", func() {
			BeforeEach(func() {
				metaServers = map[string][]string{}
				legacyServers = map[string]string{
					"z1/doppler_z1/2": "11.21.31.41",
					"z1/doppler_z1/3": "21.22.23.24",
				}
			})

			It("returns udp dopplers from legacy endpoint", func() {
				event := finder.Next()
				Expect(event.UDPDopplers).To(ConsistOf(
					fmt.Sprintf("11.21.31.41:%d", legacyPort),
					fmt.Sprintf("21.22.23.24:%d", legacyPort),
				))
				Expect(event.TCPDopplers).To(BeEmpty())
				Expect(event.TLSDopplers).To(BeEmpty())
			})
		})

		Context("legacy and meta endpoints available", func() {
			BeforeEach(func() {
				metaServers = map[string][]string{
					"z1/doppler_z1/0": {"udp://9.8.7.6:3457", "tls://9.8.7.6:3458"},
				}
				legacyServers = map[string]string{
					"z1/doppler_z1/0": "9.8.7.6",
					"z1/doppler_z1/1": "21.22.23.24",
				}
			})

			It("return udp dopplers from meta and legacy endpoints", func() {
				event := finder.Next()
				Expect(event.UDPDopplers).To(HaveLen(2))
				Expect(event.UDPDopplers).To(ConsistOf("9.8.7.6:3457", fmt.Sprintf("21.22.23.24:%d", legacyPort)))
				Expect(event.TLSDopplers).To(ConsistOf("9.8.7.6:3458"))
				Expect(event.TCPDopplers).To(BeEmpty())
			})
		})
	})
})

func etcdNodes(meta map[string][]string, legacy map[string]string) (metaNode, legacyNode storeadapter.StoreNode) {
	metaNode = storeadapter.StoreNode{
		Key: dopplerservice.META_ROOT,
		Dir: true,
	}
	for doppler, addresses := range meta {
		metaNode.ChildNodes = append(metaNode.ChildNodes, makeMetaNode(doppler, addresses))
	}
	legacyNode = storeadapter.StoreNode{
		Key: dopplerservice.LEGACY_ROOT,
		Dir: true,
	}
	for doppler, ip := range legacy {
		legacyNode.ChildNodes = append(legacyNode.ChildNodes, storeadapter.StoreNode{
			Key:   path.Join(dopplerservice.LEGACY_ROOT, doppler),
			Value: []byte(ip),
		})
	}
	return metaNode, legacyNode
}

func makeMetaNode(doppler string, addresses []string) storeadapter.StoreNode {
	metaFmt := `
	{
	  "version": 1,
	  "endpoints": %s
	}`
	endpoints, err := json.Marshal(addresses)
	Expect(err).ToNot(HaveOccurred())
	return storeadapter.StoreNode{
		Key:   path.Join(dopplerservice.META_ROOT, doppler),
		Value: []byte(fmt.Sprintf(metaFmt, string(endpoints))),
	}
}

func protocolField(event dopplerservice.Event, protocol string) []string {
	protocol = strings.ToUpper(protocol)
	addrs, ok := reflect.ValueOf(event).FieldByName(protocol + "Dopplers").Interface().([]string)
	if !ok {
		return nil
	}
	return addrs
}
