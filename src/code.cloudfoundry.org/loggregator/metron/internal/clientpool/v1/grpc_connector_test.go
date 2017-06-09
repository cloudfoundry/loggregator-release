package v1_test

import (
	"errors"
	"io"
	"io/ioutil"
	"net"

	"code.cloudfoundry.org/loggregator/plumbing"

	"code.cloudfoundry.org/loggregator/metron/internal/clientpool/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type SpyFetcher struct {
	Addr   string
	Closer io.Closer
	Client plumbing.DopplerIngestor_PusherClient
}

func (f *SpyFetcher) Fetch(addr string) (conn io.Closer, client plumbing.DopplerIngestor_PusherClient, err error) {
	f.Addr = addr
	return f.Closer, f.Client, nil
}

type SpyStream struct {
	plumbing.DopplerIngestor_PusherClient
}

var _ = Describe("GRPCConnector", func() {
	Context("when first balancer returns an address", func() {
		var (
			fetcher   *SpyFetcher
			connector v1.GRPCConnector
		)

		BeforeEach(func() {
			balancers := []*v1.Balancer{
				v1.NewBalancer("z1.doppler.com:99", v1.WithLookup(func(string) ([]net.IP, error) {
					return []net.IP{net.ParseIP("10.10.10.1")}, nil
				})),
				v1.NewBalancer("doppler.com:99", v1.WithLookup(func(string) ([]net.IP, error) {
					return []net.IP{net.ParseIP("1.1.1.1")}, nil
				})),
			}
			fetcher = &SpyFetcher{
				Closer: ioutil.NopCloser(nil),
				Client: SpyStream{},
			}
			connector = v1.MakeGRPCConnector(fetcher, balancers)
		})

		It("fetches a client with the given address", func() {
			connector.Connect()
			Expect(fetcher.Addr).To(Equal("10.10.10.1:99"))
		})

		It("returns the connection as a closer and client", func() {
			closer, client, err := connector.Connect()

			Expect(err).ToNot(HaveOccurred())
			Expect(closer).To(Equal(fetcher.Closer))
			Expect(client).To(Equal(fetcher.Client))
		})
	})

	Context("when first balancer does not return an address", func() {
		It("dials the next balancer", func() {
			balancers := []*v1.Balancer{
				v1.NewBalancer("z1.doppler.com:99", v1.WithLookup(func(string) ([]net.IP, error) {
					return nil, errors.New("Should not get here")
				})),
				v1.NewBalancer("doppler.com:99", v1.WithLookup(func(string) ([]net.IP, error) {
					return []net.IP{net.ParseIP("1.1.1.1")}, nil
				})),
			}
			fetcher := &SpyFetcher{
				Closer: ioutil.NopCloser(nil),
				Client: SpyStream{},
			}
			connector := v1.MakeGRPCConnector(fetcher, balancers)

			connector.Connect()
			Expect(fetcher.Addr).To(Equal("1.1.1.1:99"))
		})
	})

	Context("when the none balancer return anything", func() {
		It("returns an error", func() {
			balancers := []*v1.Balancer{
				v1.NewBalancer("z1.doppler.com:99", v1.WithLookup(func(string) ([]net.IP, error) {
					return nil, errors.New("Should not get here")
				})),
				v1.NewBalancer("doppler.com:99", v1.WithLookup(func(string) ([]net.IP, error) {
					return nil, errors.New("Should not get here")
				})),
			}
			fetcher := &SpyFetcher{
				Closer: ioutil.NopCloser(nil),
				Client: SpyStream{},
			}
			connector := v1.MakeGRPCConnector(fetcher, balancers)

			_, _, err := connector.Connect()
			Expect(err).To(HaveOccurred())
		})
	})
})
