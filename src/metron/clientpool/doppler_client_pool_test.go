package clientpool_test

import (
	"errors"
	"metron/clientpool"
	"metron/clientpool/fakes"
	"strings"

	steno "github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/loggregatorclient"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
)

//go:generate counterfeiter -o fakess/client.go ../../github.com/cloudfoundry/loggregatorlib/loggregatorclient Client
var _ = Describe("DopplerPool", func() {
	var (
		pool          *clientpool.DopplerPool
		clientFactory func(logger *steno.Logger, u string) (loggregatorclient.Client, error)
		logger        *steno.Logger

		port int

		preferredServers map[string]string
		allServers       map[string]string

		preferredLegacy map[string]string
		allLegacy       map[string]string
	)

	BeforeEach(func() {
		logger = steno.NewLogger("TestLogger")

		port = 5000 + config.GinkgoConfig.ParallelNode*100

		clientFactory = fakeClientFactory

		preferredServers = map[string]string{
			"a": "udp://pahost:paport",
			"b": "tls://pbhost:pbport",
		}
		allServers = map[string]string{
			"a": "udp://ahost:aport",
			"b": "udp://bhost:bport",
		}

		preferredLegacy = map[string]string{
			"a": "udp://plahost:plaport",
			"b": "tls://plbhost:plbport",
		}
		allLegacy = map[string]string{
			"a": "udp://lahost:laport",
			"b": "udp://lbhost:lbport",
		}
	})

	JustBeforeEach(func() {
		pool = clientpool.NewDopplerPool(logger, clientFactory)
	})

	Describe("Setters", func() {
		Context("non-Legacy servers", func() {
			Context("with preferred servers", func() {
				It("has only preferred servers", func() {
					pool.Set(allServers, preferredServers)
					Expect(urls(pool.Clients())).To(ConsistOf(values(preferredServers)))
				})
			})

			Context("with no preferred servers", func() {
				It("has only non-preferred servers", func() {
					pool.Set(allServers, nil)
					Expect(urls(pool.Clients())).To(ConsistOf(values(allServers)))
				})
			})

			Context("when the client factory errors", func() {
				BeforeEach(func() {
					logger = loggertesthelper.Logger()
					loggertesthelper.TestLoggerSink.Clear()

					clientFactory = func(_ *steno.Logger, _ string) (loggregatorclient.Client, error) {
						return nil, errors.New("boom")
					}
				})

				It("does not include the client", func() {
					pool.Set(allServers, nil)
					Expect(pool.Clients()).To(BeEmpty())
					Expect(loggertesthelper.TestLoggerSink.LogContents()).To(ContainSubstring("Invalid url"))
				})
			})

			Context("with Legacy Servers", func() {
				It("ignores them", func() {
					pool.Set(allServers, preferredServers)
					pool.SetLegacy(allLegacy, preferredLegacy)

					Expect(urls(pool.Clients())).To(ConsistOf(values(preferredServers)))
				})

				Context("with non-overlapping servers", func() {
					It("returns a mix of legacy and non-legacy", func() {
						allLegacy["c"] = "udp://lchost:lcport"
						pool.Set(allServers, preferredServers)
						pool.SetLegacy(allLegacy, preferredLegacy)

						Expect(urls(pool.Clients())).To(ConsistOf(values(preferredServers)))
					})
				})
			})

			Context("only Legacy servers", func() {
				Context("with preferred servers", func() {
					It("has only preferred servers", func() {
						pool.SetLegacy(allLegacy, preferredLegacy)
						Expect(urls(pool.Clients())).To(ConsistOf(values(preferredLegacy)))
					})
				})

				Context("with no preferred servers", func() {
					It("has only non-preferred servers", func() {
						pool.SetLegacy(allLegacy, nil)
						Expect(urls(pool.Clients())).To(ConsistOf(values(allLegacy)))
					})
				})

				Context("when the client factory errors", func() {
					BeforeEach(func() {
						logger = loggertesthelper.Logger()
						loggertesthelper.TestLoggerSink.Clear()

						clientFactory = func(_ *steno.Logger, _ string) (loggregatorclient.Client, error) {
							return nil, errors.New("boom")
						}
					})

					It("does not include the client", func() {
						pool.SetLegacy(allLegacy, nil)
						Expect(pool.Clients()).To(BeEmpty())
						Expect(loggertesthelper.TestLoggerSink.LogContents()).To(ContainSubstring("Invalid url"))
					})
				})
			})
		})

		Context("when a server no longer exists", func() {
			var fakeClient *fakes.FakeClient

			BeforeEach(func() {
				fakeClient = newFakeClient("udp", "host:port")
				clientFactory = func(_ *steno.Logger, _ string) (loggregatorclient.Client, error) {
					return fakeClient, nil
				}
			})

			It("the client is stopped", func() {
				s := map[string]string{
					"a": "udp://host:port",
				}

				pool.Set(s, nil)
				Expect(fakeClient.StopCallCount()).To(Equal(0))
				pool.Set(nil, nil)
				Expect(fakeClient.StopCallCount()).To(Equal(1))
			})
		})

		Context("with identical server data", func() {
			It("reuses the Client", func() {
				aServers := map[string]string{
					"a": "a://b:c",
				}
				pool.Set(aServers, nil)
				clientsA := pool.Clients()
				Expect(clientsA).To(HaveLen(1))

				mixedServers := map[string]string{
					"a": "a://b:c",
					"c": "d://e:f",
				}
				pool.Set(mixedServers, nil)
				clientsB := pool.Clients()
				Expect(clientsB).To(HaveLen(2))
				Expect(clientsB).To(ContainElement(clientsA[0]))
			})
		})
	})

	Describe("Clients", func() {
		Context("with no servers", func() {
			It("returns an empty list", func() {
				Expect(pool.Clients()).To(HaveLen(0))
			})
		})
	})

	Describe("RandomClient", func() {
		Context("with a non-empty client pool", func() {
			It("chooses a client with roughly uniform distribution", func() {
				s := map[string]string{
					"1": "udp://host:1",
					"2": "udp://host:2",
					"3": "udp://host:3",
					"4": "udp://host:4",
					"5": "udp://host:5",
				}

				pool.Set(s, nil)
				counts := make(map[loggregatorclient.Client]int)
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
				Expect(err).To(Equal(clientpool.ErrorEmptyClientPool))
			})
		})
	})
})

func urls(clients []loggregatorclient.Client) []string {
	result := make([]string, 0, len(clients))
	for _, c := range clients {
		result = append(result, c.Scheme()+"://"+c.Address())
	}
	return result
}

func values(m map[string]string) []string {
	result := make([]string, 0, len(m))
	for _, c := range m {
		result = append(result, c)
	}
	return result
}

func fakeClientFactory(logger *steno.Logger, u string) (loggregatorclient.Client, error) {
	i := strings.Index(u, "://")
	return newFakeClient(u[:i], u[i+3:]), nil
}

func newFakeClient(proto, addr string) *fakes.FakeClient {
	c := fakes.FakeClient{}
	c.SchemeReturns(proto)
	c.AddressReturns(addr)
	return &c
}
