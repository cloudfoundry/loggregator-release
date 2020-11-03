package throughputlb_test

import (
	"time"

	throughputlb "code.cloudfoundry.org/loggregator/grpc-throughputlb"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ThroughputLoadBalancer", func() {
	var (
		maxRequests int = 10
		numAddrs    int = 5
	)

	It("conforms to the gRPC Balancer interface", func() {
		// This test will fail at compilation if the ThroughputLoadBalancer
		// does not satisfy the grpc.Balancer interface.
		var i grpc.Balancer = throughputlb.NewThroughputLoadBalancer(maxRequests, numAddrs)
		_ = i
	})

	It("sends a notification of an available address", func() {
		lb := throughputlb.NewThroughputLoadBalancer(maxRequests, numAddrs)
		err := lb.Start("127.0.0.1:3000", grpc.BalancerConfig{})
		Expect(err).ToNot(HaveOccurred())

		Expect(lb.Notify()).To(Receive(Equal([]grpc.Address{
			{Addr: "127.0.0.1:3000", Metadata: 0},
			{Addr: "127.0.0.1:3000", Metadata: 1},
			{Addr: "127.0.0.1:3000", Metadata: 2},
			{Addr: "127.0.0.1:3000", Metadata: 3},
			{Addr: "127.0.0.1:3000", Metadata: 4},
		})))
	})

	It("blocks if no addresses are available and blocking wait is true", func() {
		lb := throughputlb.NewThroughputLoadBalancer(maxRequests, numAddrs)
		err := lb.Start("127.0.0.1:3000", grpc.BalancerConfig{})
		Expect(err).ToNot(HaveOccurred())

		done := make(chan struct{})
		go func() {
			_, _, _ = lb.Get(context.Background(), grpc.BalancerGetOptions{BlockingWait: true})
			close(done)
		}()

		Consistently(done).ShouldNot(BeClosed())
	})

	It("returns an error if no addresses are available when blocking wait is false", func() {
		lb := throughputlb.NewThroughputLoadBalancer(maxRequests, numAddrs)
		err := lb.Start("127.0.0.1:3000", grpc.BalancerConfig{})
		Expect(err).ToNot(HaveOccurred())

		_, _, err = lb.Get(context.Background(), grpc.BalancerGetOptions{BlockingWait: false})
		Expect(err).To(MatchError(grpc.Errorf(codes.Unavailable, "there is no address available")))
	})

	It("returns an error if the only address enters a down state", func() {
		lb := throughputlb.NewThroughputLoadBalancer(maxRequests, numAddrs)
		err := lb.Start("127.0.0.1:3000", grpc.BalancerConfig{})
		Expect(err).ToNot(HaveOccurred())

		var addrs []grpc.Address
		Expect(lb.Notify()).To(Receive(&addrs))

		down := lb.Up(addrs[0])
		down(nil)

		_, _, err = lb.Get(context.Background(), grpc.BalancerGetOptions{})
		Expect(err).To(MatchError(grpc.Errorf(codes.Unavailable, "there is no address available")))
	})

	It("balances requests among all addrs", func() {
		lb := throughputlb.NewThroughputLoadBalancer(maxRequests, numAddrs)
		err := lb.Start("127.0.0.1:3000", grpc.BalancerConfig{})
		Expect(err).ToNot(HaveOccurred())

		var addrs []grpc.Address
		Expect(lb.Notify()).To(Receive(&addrs))

		for _, a := range addrs {
			lb.Up(a)
		}

		for i := 0; i < int(numAddrs); i++ {
			addr, _, err := lb.Get(context.Background(), grpc.BalancerGetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(addr).To(Equal(addrs[i]))
		}
	})

	It("chooses the least used address", func() {
		lb := throughputlb.NewThroughputLoadBalancer(maxRequests, numAddrs)
		err := lb.Start("127.0.0.1:3000", grpc.BalancerConfig{})
		Expect(err).ToNot(HaveOccurred())

		var addrs []grpc.Address
		Expect(lb.Notify()).To(Receive(&addrs))

		for _, a := range addrs {
			lb.Up(a)
		}

		putters := make([][]func(), numAddrs)

		By("maxing out active requests")
		for i := 0; i < numAddrs; i++ {
			for x := 0; x < maxRequests; x++ {
				a, putter, err := lb.Get(context.Background(), grpc.BalancerGetOptions{})
				Expect(err).ToNot(HaveOccurred())

				putters[a.Metadata.(int)] = append(putters[a.Metadata.(int)], putter)
			}
		}

		By("releasing all #1 and #3 addrs")
		for _, p := range putters[1] {
			p()
		}

		for _, p := range putters[3] {
			p()
		}

		By("getting more addrs")
		addr, _, err := lb.Get(context.Background(), grpc.BalancerGetOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(addr).To(Equal(addrs[1]))

		addr, _, err = lb.Get(context.Background(), grpc.BalancerGetOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(addr).To(Equal(addrs[3]))

		addr, _, err = lb.Get(context.Background(), grpc.BalancerGetOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(addr).To(Equal(addrs[1]))

		addr, _, err = lb.Get(context.Background(), grpc.BalancerGetOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(addr).To(Equal(addrs[3]))
	})

	It("is goroutine safe", func() {
		lb := throughputlb.NewThroughputLoadBalancer(maxRequests, numAddrs)
		err := lb.Start("127.0.0.1:3000", grpc.BalancerConfig{})
		Expect(err).ToNot(HaveOccurred())

		getter := func() {
			defer GinkgoRecover()

			for {
				_, put, err := lb.Get(context.Background(), grpc.BalancerGetOptions{BlockingWait: true})
				Expect(err).ToNot(HaveOccurred())

				put()
			}
		}

		upper := func() {
			defer GinkgoRecover()

			for {
				addrs := <-lb.Notify()

				for i, a := range addrs {
					down := lb.Up(a)

					if i == 2 {
						go down(nil)
					}
				}
			}
		}

		for i := 0; i < 50; i++ {
			go getter()
		}

		go upper()

		time.Sleep(time.Second)
	})
})
