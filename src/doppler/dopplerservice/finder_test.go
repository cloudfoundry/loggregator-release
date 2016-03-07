package dopplerservice_test

import (
	"doppler/dopplerservice"
	"encoding/json"
	"errors"
	"fmt"
	"path"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/storeadapter"
)

var _ = Describe("Finder", func() {

	var (
		testLogger           *gosteno.Logger
		preferredProtocol    string
		mockStoreAdapter     *mockStoreAdapter
		port                 int
		preferredDopplerZone string

		finder *dopplerservice.Finder
	)

	BeforeEach(func() {
		preferredDopplerZone = ""
		preferredProtocol = "tls"
		mockStoreAdapter = newMockStoreAdapter()
		port = 1234
		testLogger = gosteno.NewLogger("TestLogger")
	})

	JustBeforeEach(func() {
		finder = dopplerservice.NewFinder(mockStoreAdapter, port, preferredProtocol, preferredDopplerZone, testLogger)
		Expect(finder).ToNot(BeNil())
	})

	Describe("WebsocketServers", func() {
		var (
			metaNode, legacyNode storeadapter.StoreNode
			metaServers          map[string][]string
			legacyServers        map[string]string
			expectedServers      []string
		)

		BeforeEach(func() {
			port = 8081
			preferredProtocol = "udp"
			preferredDopplerZone = ""
			expectedServers = []string{
				"11.21.31.41:8081",
				"21.22.23.24:8081",
			}

			metaServers = map[string][]string{
				"z1/doppler_z1/0": []string{"udp://1.2.3.4:3457"},
				"z1/doppler_z1/1": []string{"udp://9.8.7.6:3457", "tls://9.8.7.6:3458"},
				"z1/doppler_z1/2": []string{"udp://11.21.31.41:3457", "tls://11.21.31.41:3458"},
				"z1/doppler_z1/4": []string{"udp://31.32.33.34:3457", "tls://31.32.33.34:3458"},
			}

			legacyServers = map[string]string{
				"z1/doppler_z1/2": "11.21.31.41",
				"z1/doppler_z1/3": "21.22.23.24",
			}

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

		Context("valid server urls", func() {
			It("returns Doppler's websocket connection url", func() {
				websocketServers := finder.WebsocketServers()

				for _, server := range expectedServers {
					Expect(websocketServers).To(ContainElement(server))
				}
			})
		})

		Context("invalid server urls", func() {

			BeforeEach(func() {
				legacyServers["z1/doppler_z1/5"] = "%"
			})

			It("are ignored", func() {
				websocketServers := finder.WebsocketServers()
				Expect(websocketServers).To(HaveLen(len(expectedServers)))
				for _, server := range expectedServers {
					Expect(websocketServers).To(ContainElement(server))
				}
			})
		})
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
				metaNode, legacyNode = etcdNodes(map[string][]string{
					"z1/doppler_z1/0": []string{"udp://1.2.3.4:567"},
					"z1/doppler_z1/1": []string{"tls://9.8.7.6:555"},
					"z1/doppler_z1/2": []string{"xyz://9.8.7.6:543", "tca://9.8.7.6:555"},
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
				metaNode, legacyNode = etcdNodes(map[string][]string{
					"z1/doppler_z1/0": []string{"udp://1.2.3.4:567"},
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

		//TODO: test map with empty slice of addresses
		Context("with data in the meta node", func() {
			BeforeEach(func() {
				preferredProtocol = "tls"
				metaNode, legacyNode = etcdNodes(map[string][]string{
					"z1/doppler_z1/0": []string{"udp://1.2.3.4:567"},
					"z1/doppler_z1/1": []string{"udp://9.8.7.6:543", "tls://9.8.7.6:555"},
				}, nil)
			})

			It("returns the endpoint of each doppler", func() {
				event := finder.Next()
				Expect(event.UDPDopplers).To(Equal([]string{"1.2.3.4:567"}))
				Expect(event.TLSDopplers).To(Equal([]string{"9.8.7.6:555"}))
			})
		})

		Context("with data in the legacy node", func() {
			BeforeEach(func() {
				metaNode, legacyNode = etcdNodes(nil, map[string]string{
					"z1/doppler_z1/0": "1.2.3.4",
					"z1/doppler_z1/1": "5.6.7.8",
				})
			})

			It("returns the endpoint of each doppler, with the protocol and port", func() {
				event := finder.Next()
				firstURL := fmt.Sprintf("1.2.3.4:%d", port)
				secondURL := fmt.Sprintf("5.6.7.8:%d", port)
				Expect(event.UDPDopplers).To(ConsistOf(firstURL, secondURL))
				Expect(event.TLSDopplers).To(BeEmpty())
			})
		})

		Context("with data in both nodes", func() {
			BeforeEach(func() {
				preferredProtocol = "udp"
				metaNode, legacyNode = etcdNodes(map[string][]string{
					"z1/doppler_z1/0": []string{"udp://1.2.3.4:567"},
					"z1/doppler_z1/1": []string{"udp://9.8.7.6:543", "tls://9.8.7.6:555"},
					"z1/doppler_z1/2": []string{"tls://11.21.31.41:1234"},
					"z1/doppler_z1/4": []string{"tls://31.32.33.34:1234"},
				}, map[string]string{
					"z1/doppler_z1/2": "11.21.31.41",
					"z1/doppler_z1/3": "21.22.23.24",
				})
			})

			It("merges the nodes", func() {
				expectedUDP := []string{
					"1.2.3.4:567",
					"9.8.7.6:543",
					fmt.Sprintf("11.21.31.41:%d", port),
					fmt.Sprintf("21.22.23.24:%d", port),
				}
				expectedTLS := []string{
					"31.32.33.34:1234",
				}
				event := finder.Next()
				Expect(event.UDPDopplers).To(ConsistOf(expectedUDP))
				Expect(event.TLSDopplers).To(ConsistOf(expectedTLS))
			})
		})

		Context("with preferred dopplers in the pool", func() {
			BeforeEach(func() {
				preferredDopplerZone = "z3"
				metaNode, legacyNode = etcdNodes(map[string][]string{
					"z1/doppler_z1/0": []string{"udp://1.2.3.4:567"},
					"z3/doppler_z3/2": []string{"tls://11.21.31.41:1234"},
				}, map[string]string{
					"z2/doppler_z2/2": "11.21.31.41",
					"z3/doppler_z3/3": "21.22.23.24",
				})
			})

			It("skips non-preferred doppler instances", func() {
				expectedUDP := []string{fmt.Sprintf("21.22.23.24:%d", port)}
				expectedTLS := []string{"11.21.31.41:1234"}

				event := finder.Next()
				Expect(event.UDPDopplers).To(Equal(expectedUDP))
				Expect(event.TLSDopplers).To(Equal(expectedTLS))
			})

		})

		Context("without any preferred dopplers in the pool", func() {
			BeforeEach(func() {
				preferredDopplerZone = "z4"
				metaNode, legacyNode = etcdNodes(map[string][]string{
					"z1/doppler_z1/0": []string{"udp://1.2.3.4:567"},
					"z3/doppler_z3/2": []string{"tls://11.21.31.41:1234"},
				}, map[string]string{
					"z2/doppler_z2/2": "11.21.31.41",
					"z3/doppler_z3/3": "21.22.23.24",
				})
			})

			It("returns all available doppler instances", func() {
				expectedUDP := []string{
					"1.2.3.4:567",
					fmt.Sprintf("11.21.31.41:%d", port),
					fmt.Sprintf("21.22.23.24:%d", port),
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

		Context("meta endpoints", func() {
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
				})
			})

			Context("when a node is updated", func() {
				BeforeEach(func() {
					preferredProtocol = "tls"
					metaNode = makeMetaNode("z1/doppler_z1/0", []string{"udp://1.2.3.4:567"})
				})

				JustBeforeEach(func() {
					// Ignore the startup event
					_ = finder.Next()

					updateNode := makeMetaNode("z1/doppler_z1/0", []string{"tls://1.2.3.4:555", "udp://1.2.3.4:567"})
					metaEvents <- storeadapter.WatchEvent{
						Type:     storeadapter.UpdateEvent,
						Node:     &updateNode,
						PrevNode: &metaNode,
					}
				})

				It("returns the updated endpoints", func() {
					event := finder.Next()
					Expect(event.TLSDopplers).To(Equal([]string{"1.2.3.4:555"}))
					Expect(event.UDPDopplers).To(BeEmpty())
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
						Type:     storeadapter.UpdateEvent,
						Node:     &node,
						PrevNode: &node,
					}
					Eventually(done).Should(BeClosed())
				})
			})
		})

		Context("legacy endpoints", func() {

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
					Expect(event.UDPDopplers).To(Equal([]string{fmt.Sprintf("1.2.3.4:%d", port)}))
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
					Expect(event.UDPDopplers).To(Equal([]string{fmt.Sprintf("5.6.7.8:%d", port)}))
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
						Type:     storeadapter.UpdateEvent,
						Node:     &node,
						PrevNode: &node,
					}
					Eventually(done).Should(BeClosed())
				})
			})
		})
	})

	Context("TLS Preferred Protocol on Metron", func() {

		var (
			metaNode, legacyNode storeadapter.StoreNode
			metaServers          map[string][]string
			legacyServers        map[string]string
		)

		BeforeEach(func() {
			port = 9999
			preferredProtocol = "tls"

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

		Context("Dopplers advertise on legacy root only", func() {

			BeforeEach(func() {
				metaServers = map[string][]string{}
				legacyServers = map[string]string{
					"z1/doppler_z1/2": "11.21.31.41",
					"z1/doppler_z1/3": "21.22.23.24",
				}
			})

			It("returns udp dopplers from legacy endpoint", func() {
				event := finder.Next()
				Expect(event.UDPDopplers).To(HaveLen(2))
				Expect(event.UDPDopplers).To(ContainElement("11.21.31.41:9999"))
				Expect(event.UDPDopplers).To(ContainElement("21.22.23.24:9999"))
			})
		})

		Context("Dopplers advertise UDP on meta and legacy root", func() {

			BeforeEach(func() {
				metaServers = map[string][]string{
					"z1/doppler_z1/0": []string{"udp://9.8.7.6:3457"},
				}
				legacyServers = map[string]string{
					"z1/doppler_z1/0": "9.8.7.6",
					"z1/doppler_z1/1": "21.22.23.24",
				}
			})

			It("returns udp dopplers from meta endpoint", func() {
				event := finder.Next()
				Expect(event.UDPDopplers).To(HaveLen(2))
				Expect(event.UDPDopplers).To(ContainElement("9.8.7.6:3457"))
				Expect(event.UDPDopplers).To(ContainElement("21.22.23.24:9999"))
			})
		})

		Context("Dopplers advertise TLS/UDP on meta and legacy root", func() {

			BeforeEach(func() {
				metaServers = map[string][]string{
					"z1/doppler_z1/0": []string{"udp://9.8.7.6:3457", "tls://9.8.7.6:3458"},
				}
				legacyServers = map[string]string{
					"z1/doppler_z1/0": "9.8.7.6",
					"z1/doppler_z1/1": "21.22.23.24",
				}
			})

			It("returns udp dopplers from meta endpoint", func() {
				event := finder.Next()
				Expect(event.UDPDopplers).To(HaveLen(1))
				Expect(event.UDPDopplers).To(ContainElement("21.22.23.24:9999"))
				Expect(event.TLSDopplers).To(HaveLen(1))
				Expect(event.TLSDopplers).To(ContainElement("9.8.7.6:3458"))
			})
		})

		Context("Dopplers advertise UDP and an unsupported protocol", func() {
			BeforeEach(func() {
				metaServers = map[string][]string{
					"z1/doppler_z1/0": []string{"https://1.2.3.4/foo", "udp://1.2.3.4:5678", "wss://1.2.3.4/bar"},
				}
				legacyServers = map[string]string{}
			})

			It("returns the supported address for that doppler", func() {
				event := finder.Next()
				Expect(event.UDPDopplers).To(ConsistOf("1.2.3.4:5678"))
			})
		})
	})

	Context("UDP Preferred Protocol on Metron", func() {
		var (
			metaServers   map[string][]string
			legacyServers map[string]string
		)

		BeforeEach(func() {
			port = 9999
			preferredProtocol = "udp"

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
				Expect(event.UDPDopplers).To(HaveLen(2))
				Expect(event.UDPDopplers).To(ContainElement(fmt.Sprintf("11.21.31.41:%d", port)))
				Expect(event.UDPDopplers).To(ContainElement(fmt.Sprintf("21.22.23.24:%d", port)))
			})
		})

		Context("legacy and meta endpoints available", func() {
			BeforeEach(func() {
				metaServers = map[string][]string{
					"z1/doppler_z1/0": []string{"udp://9.8.7.6:3457", "tls://9.8.7.6:3458"},
				}
				legacyServers = map[string]string{
					"z1/doppler_z1/0": "9.8.7.6",
					"z1/doppler_z1/1": "21.22.23.24",
				}
			})

			It("return udp dopplers from meta and legacy endpoints", func() {
				event := finder.Next()
				Expect(event.UDPDopplers).To(HaveLen(2))
				Expect(event.UDPDopplers).To(ConsistOf("9.8.7.6:3457", fmt.Sprintf("21.22.23.24:%d", port)))
				Expect(event.TLSDopplers).To(HaveLen(0))
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
