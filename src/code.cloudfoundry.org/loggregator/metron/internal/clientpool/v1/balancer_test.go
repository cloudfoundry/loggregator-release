package v1_test

import (
	"errors"
	"math/rand"
	"net"

	"code.cloudfoundry.org/loggregator/metron/internal/clientpool/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Balancer", func() {
	It("returns a random IP address if lookup has IPs", func() {
		rand.Seed(1)

		f := func(addr string) ([]net.IP, error) {
			return []net.IP{
				net.ParseIP("10.10.10.1"),
				net.ParseIP("10.10.10.2"),
				net.ParseIP("10.10.10.3"),
				net.ParseIP("10.10.10.4"),
			}, nil
		}

		addr := "some-addr:8082"
		balancer := v1.NewBalancer(addr, v1.WithLookup(f))

		NextIP := func() string {
			ip, _ := balancer.NextHostPort()
			return ip
		}
		Expect(NextIP()).To(Equal("10.10.10.3:8082"))
		Expect(NextIP()).To(Equal("10.10.10.4:8082"))
		Expect(NextIP()).To(Equal("10.10.10.2:8082"))
		Expect(NextIP()).To(Equal("10.10.10.4:8082"))
	})

	It("returns an error if lookup fails", func() {
		f := func(addr string) ([]net.IP, error) {
			return nil, errors.New("some-error")
		}
		addr := "some-addr:8082"
		balancer := v1.NewBalancer(addr, v1.WithLookup(f))

		_, err := balancer.NextHostPort()
		Expect(err).To(HaveOccurred())
	})

	It("returns an error if lookup is empty", func() {
		f := func(addr string) ([]net.IP, error) {
			return []net.IP{}, nil
		}
		addr := "some-addr:8082"
		balancer := v1.NewBalancer(addr, v1.WithLookup(f))

		_, err := balancer.NextHostPort()
		Expect(err).To(HaveOccurred())
	})

	It("returns an error with invalid hostname", func() {
		f := func(addr string) ([]net.IP, error) {
			panic("Never should be here")
		}
		addr := "some-addr/qwe34523475itaysdgp:oauei4h:8082"
		balancer := v1.NewBalancer(addr, v1.WithLookup(f))

		_, err := balancer.NextHostPort()
		Expect(err).To(HaveOccurred())
	})
})
