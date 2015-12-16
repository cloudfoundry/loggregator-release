package iprange_test

import (
	"doppler/iprange"
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("IPRange", func() {
	Describe("ValidateIpAddresses", func() {
		It("recognizes a valid IP address range", func() {
			ranges := []iprange.IPRange{iprange.IPRange{Start: "127.0.2.2", End: "127.0.2.4"}}
			err := iprange.ValidateIpAddresses(ranges)
			Expect(err).NotTo(HaveOccurred())
		})

		It("validates the start address", func() {
			ranges := []iprange.IPRange{iprange.IPRange{Start: "127.0.2.2.1", End: "127.0.2.4"}}
			err := iprange.ValidateIpAddresses(ranges)
			Expect(err).To(MatchError("Invalid IP Address for Blacklist IP Range: 127.0.2.2.1"))
		})

		It("validates the end address", func() {
			ranges := []iprange.IPRange{iprange.IPRange{Start: "127.0.2.2", End: "127.0.2.4.3"}}
			err := iprange.ValidateIpAddresses(ranges)
			Expect(err).To(HaveOccurred())
		})

		It("validates all given IP addresses", func() {
			ranges := []iprange.IPRange{
				iprange.IPRange{Start: "127.0.2.2", End: "127.0.2.4"},
				iprange.IPRange{Start: "127.0.2.2", End: "127.0.2.4.5"},
			}
			err := iprange.ValidateIpAddresses(ranges)
			Expect(err).To(HaveOccurred())
		})

		It("validates that start IP is before end IP", func() {
			ranges := []iprange.IPRange{iprange.IPRange{Start: "10.10.10.10", End: "10.8.10.12"}}
			err := iprange.ValidateIpAddresses(ranges)
			Expect(err).To(MatchError("Invalid Blacklist IP Range: Start 10.10.10.10 has to be before End 10.8.10.12"))
		})

		It("accepts start and end as the same", func() {
			ranges := []iprange.IPRange{iprange.IPRange{Start: "127.0.2.2", End: "127.0.2.2"}}
			err := iprange.ValidateIpAddresses(ranges)
			Expect(err).NotTo(HaveOccurred())
		})

	})

	Describe("IpOutsideOfRanges", func() {
		var mockIPResolver *mockIPResolver

		BeforeEach(func() {
			mockIPResolver = newMockIPResolver()
		})

		It("parses the IP address properly", func() {
			ranges := []iprange.IPRange{iprange.IPRange{Start: "127.0.1.2", End: "127.0.3.4"}}

			for _, ipTest := range ipTests {
				parsedURL, _ := url.Parse(ipTest.url)
				outOfRange, err := iprange.IpOutsideOfRanges(*parsedURL, ranges, mockIPResolver)
				Expect(err).NotTo(HaveOccurred())
				Expect(outOfRange).To(Equal(ipTest.output), fmt.Sprintf("Wrong output for url: %s", ipTest.url))
			}
		})

		It("returns error on malformatted URL", func() {
			ranges := []iprange.IPRange{iprange.IPRange{Start: "127.0.2.2", End: "127.0.2.4"}}

			for _, testUrl := range malformattedURLs {
				parsedURL, _ := url.Parse(testUrl.url)
				_, err := iprange.IpOutsideOfRanges(*parsedURL, ranges, mockIPResolver)
				if err == nil {
					GinkgoT().Fatal(fmt.Sprintf("There should be an error about malformatted URL for %s", testUrl))
				}
			}
		})

		It("always returns true when IP ranges is nil or empty", func() {
			ranges := []iprange.IPRange{}

			close(mockIPResolver.ResolveIPAddrOutput.ret1)

			parsedURL, _ := url.Parse("https://127.0.0.1")
			mockIPResolver.ResolveIPAddrOutput.ret0 <- &net.IPAddr{IP: net.ParseIP("127.0.0.1")}
			outSideOfRange, err := iprange.IpOutsideOfRanges(*parsedURL, ranges, mockIPResolver)
			Expect(err).NotTo(HaveOccurred())
			Expect(outSideOfRange).To(BeTrue())

			ranges = nil
			mockIPResolver.ResolveIPAddrOutput.ret0 <- &net.IPAddr{IP: net.ParseIP("127.0.0.1")}
			outSideOfRange, err = iprange.IpOutsideOfRanges(*parsedURL, ranges, mockIPResolver)
			Expect(err).NotTo(HaveOccurred())
			Expect(outSideOfRange).To(BeTrue())
		})

		It("resolves IP addresses", func() {
			ranges := []iprange.IPRange{iprange.IPRange{Start: "127.0.0.0", End: "127.0.0.4"}}

			parsedURL, _ := url.Parse("syslog://vcap.me:3000?app=great")
			mockIPResolver.ResolveIPAddrOutput.ret0 <- &net.IPAddr{IP: net.ParseIP("127.0.0.1")}
			mockIPResolver.ResolveIPAddrOutput.ret1 <- nil
			outSideOfRange, err := iprange.IpOutsideOfRanges(*parsedURL, ranges, mockIPResolver)
			Expect(err).NotTo(HaveOccurred())
			Expect(outSideOfRange).To(BeFalse())

			parsedURL, _ = url.Parse("syslog://localhost:3000?app=great")
			mockIPResolver.ResolveIPAddrOutput.ret0 <- &net.IPAddr{IP: net.ParseIP("127.0.0.1")}
			mockIPResolver.ResolveIPAddrOutput.ret1 <- nil
			outSideOfRange, err = iprange.IpOutsideOfRanges(*parsedURL, ranges, mockIPResolver)
			Expect(err).NotTo(HaveOccurred())
			Expect(outSideOfRange).To(BeFalse())

			parsedURL, _ = url.Parse("syslog://doesNotExist.local:3000?app=great")
			mockIPResolver.ResolveIPAddrOutput.ret0 <- nil
			mockIPResolver.ResolveIPAddrOutput.ret1 <- errors.New("Resolving host failed: foo")
			outSideOfRange, err = iprange.IpOutsideOfRanges(*parsedURL, ranges, mockIPResolver)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Resolving host failed: "))
		})

		It("resolves IP addresses with backoff retries", func() {
			ranges := []iprange.IPRange{{Start: "127.0.0.0", End: "127.0.0.4"}}

			parsedURL, err := url.Parse("syslog://willExistSoon.local:3000?app=great")
			Expect(err).NotTo(HaveOccurred())

			for i := 0; i < 2; i++ {
				mockIPResolver.ResolveIPAddrOutput.ret0 <- nil
				mockIPResolver.ResolveIPAddrOutput.ret1 <- errors.New("Resolving host failed: foo")
			}
			mockIPResolver.ResolveIPAddrOutput.ret0 <- &net.IPAddr{IP: net.ParseIP("127.0.0.1")}
			mockIPResolver.ResolveIPAddrOutput.ret1 <- nil

			backoffs := []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
			outSideOfRange, err := iprange.IpOutsideOfRanges(*parsedURL, ranges, mockIPResolver, backoffs...)
			Expect(err).NotTo(HaveOccurred())
			Expect(outSideOfRange).To(BeFalse())
			Eventually(mockIPResolver.ResolveIPAddrOutput.ret0).Should(BeEmpty())
			Eventually(mockIPResolver.ResolveIPAddrOutput.ret1).Should(BeEmpty())
			Expect(mockIPResolver.ResolveIPAddrCalled).To(HaveLen(3))
		})
	})

})

var ipTests = []struct {
	url    string
	output bool
}{
	{"http://127.0.0.1", true},
	{"http://127.0.1.1", true},
	{"http://127.0.3.5", true},
	{"http://127.0.2.2", false},
	{"http://127.0.2.3", false},
	{"http://127.0.2.4", false},
	{"https://127.0.1.1", true},
	{"https://127.0.2.3", false},
	{"syslog://127.0.1.1", true},
	{"syslog://127.0.2.3", false},
	{"syslog://127.0.1.1:3000", true},
	{"syslog://127.0.2.3:3000", false},
	{"syslog://127.0.1.1:3000/test", true},
	{"syslog://127.0.2.3:3000/test", false},
	{"syslog://127.0.1.1:3000?app=great", true},
	{"syslog://127.0.2.3:3000?app=great", false},
	{"syslog://127.0.2.3:3000?app=great", false},
}

var malformattedURLs = []struct {
	url string
}{
	{"127.0.0.1:300/new"},
	{"syslog:127.0.0.1:300/new"},
	{"<nil>"},
}
