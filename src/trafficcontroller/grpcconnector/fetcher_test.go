package grpcconnector_test

import (
	"doppler/dopplerservice"
	"fmt"
	"net"
	"plumbing"
	"strconv"
	"strings"
	"trafficcontroller/grpcconnector"

	"google.golang.org/grpc"

	"golang.org/x/net/context"

	"github.com/cloudfoundry/gosteno"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Fetcher", func() {
	var (
		listeners    []net.Listener
		mockFinder   *mockFinder
		mockDopplerA *mockDopplerServer
		port         int

		fetcher *grpcconnector.Fetcher

		ctx context.Context

		streamReq   *plumbing.StreamRequest
		firehoseReq *plumbing.FirehoseRequest
	)

	var startMockDoppler = func() (*mockDopplerServer, string) {
		mockDoppler := newMockDopplerServer()
		lis, err := net.Listen("tcp", ":0")
		Expect(err).ToNot(HaveOccurred())
		listeners = append(listeners, lis)
		s := grpc.NewServer()
		plumbing.RegisterDopplerServer(s, mockDoppler)
		go s.Serve(lis)
		return mockDoppler, lis.Addr().String()
	}

	var extractPort = func(s string) int {
		idx := strings.LastIndexByte(s, ':') + 1
		i, err := strconv.Atoi(s[idx:])
		Expect(err).ToNot(HaveOccurred())
		return i
	}

	BeforeEach(func() {
		listeners = nil
		mockFinder = newMockFinder()
		var URIa string
		mockDopplerA, URIa = startMockDoppler()

		ctx = context.Background()

		streamReq = &plumbing.StreamRequest{
			AppID: "some-id",
		}

		firehoseReq = &plumbing.FirehoseRequest{
			SubID: "some-id",
		}

		port = extractPort(URIa)
		fetcher = grpcconnector.NewFetcher(port, mockFinder, new(gosteno.Logger))
	})

	AfterEach(func() {
		for _, lis := range listeners {
			lis.Close()
		}
	})

	var readFromReceivers = func(rxs []grpcconnector.Receiver) (chan []byte, chan error) {
		c := make(chan []byte, 100)
		e := make(chan error, 100)
		for _, rx := range rxs {
			go func(r grpcconnector.Receiver) {
				for {
					resp, err := r.Recv()
					if err != nil {
						e <- err
						return
					}
					c <- resp.Payload
				}
			}(rx)

		}
		return c, e
	}

	var chanToSlice = func(c chan []byte) (result [][]byte) {
		for {
			select {
			case r := <-c:
				result = append(result, r)
			default:
				return result
			}
		}
	}

	Describe("FetchStream()", func() {
		var fetchStreamServer = func(doppler *mockDopplerServer) plumbing.Doppler_StreamServer {
			var server plumbing.Doppler_StreamServer
			Eventually(doppler.streamInputServers).Should(Receive(&server))
			return server
		}

		var fetchStreamRxs = func() []grpcconnector.Receiver {
			var rxs []grpcconnector.Receiver
			var err error
			f := func() error {
				rxs, err = fetcher.FetchStream(ctx, streamReq)
				return err
			}
			Eventually(f).Should(Succeed())
			return rxs
		}

		Context("when Next() returns", func() {
			BeforeEach(func() {
				mockFinder.NextOutput.Ret0 <- dopplerservice.Event{
					UDPDopplers: []string{fmt.Sprintf("udp://localhost:%d", port)},
				}
			})

			It("returns new connections", func() {
				rxs := fetchStreamRxs()
				Expect(rxs).To(HaveLen(1))

				serverA := fetchStreamServer(mockDopplerA)
				serverA.Send(&plumbing.Response{Payload: []byte("data-a")})

				payloads, _ := readFromReceivers(rxs)
				Eventually(payloads).Should(HaveLen(1))
				ps := chanToSlice(payloads)
				Expect(ps).To(ConsistOf(BeEquivalentTo("data-a")))
			})

			It("closes the existing grpc connections", func() {
				rxs := fetchStreamRxs()
				Expect(rxs).To(HaveLen(1))

				_, errs := readFromReceivers(rxs)

				mockFinder.NextOutput.Ret0 <- dopplerservice.Event{
					UDPDopplers: []string{fmt.Sprintf("udp://localhost:%d", port)},
				}

				Eventually(errs).ShouldNot(BeEmpty())
			})
		})

		Context("when no doppler servers are available", func() {
			It("returns an error", func() {
				_, err := fetcher.FetchStream(ctx, streamReq)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("FetchFirehose()", func() {
		var fetchFirehoseServer = func(doppler *mockDopplerServer) plumbing.Doppler_StreamServer {
			var server plumbing.Doppler_FirehoseServer
			Eventually(doppler.firehoseInputServers).Should(Receive(&server))
			return server
		}

		var fetchFirehoseRxs = func() []grpcconnector.Receiver {
			var rxs []grpcconnector.Receiver
			var err error
			f := func() error {
				rxs, err = fetcher.FetchFirehose(ctx, firehoseReq)
				return err
			}
			Eventually(f).Should(Succeed())
			return rxs
		}

		Context("when Next() returns", func() {
			BeforeEach(func() {
				mockFinder.NextOutput.Ret0 <- dopplerservice.Event{
					UDPDopplers: []string{fmt.Sprintf("udp://localhost:%d", port)},
				}
			})

			It("returns new connections", func() {
				rxs := fetchFirehoseRxs()
				Expect(rxs).To(HaveLen(1))

				serverA := fetchFirehoseServer(mockDopplerA)
				serverA.Send(&plumbing.Response{Payload: []byte("data-a")})

				payloads, _ := readFromReceivers(rxs)
				Eventually(payloads).Should(HaveLen(1))
				ps := chanToSlice(payloads)
				Expect(ps).To(ConsistOf(BeEquivalentTo("data-a")))
			})

			It("closes the existing grpc connections", func() {
				rxs := fetchFirehoseRxs()
				Expect(rxs).To(HaveLen(1))

				_, errs := readFromReceivers(rxs)

				mockFinder.NextOutput.Ret0 <- dopplerservice.Event{
					UDPDopplers: []string{fmt.Sprintf("udp://localhost:%d", port)},
				}

				Eventually(errs).ShouldNot(BeEmpty())
			})
		})

		Context("when no doppler servers are available", func() {
			It("returns an error", func() {
				_, err := fetcher.FetchStream(ctx, streamReq)
				Expect(err).To(HaveOccurred())
			})
		})
	})

})
