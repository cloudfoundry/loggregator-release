package iprange

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

type IPResolver interface {
	ResolveIPAddr(net, addr string) (*net.IPAddr, error)
}

type IPResolverFunc func(net, addr string) (*net.IPAddr, error)

func (f IPResolverFunc) ResolveIPAddr(net, addr string) (*net.IPAddr, error) {
	return f(net, addr)
}

type IPRange struct {
	Start string
	End   string
}

func retryResolve(resolver func() (*net.IPAddr, error), backoffs ...time.Duration) (*net.IPAddr, error) {
	for _, delay := range backoffs {
		ipAddr, err := resolver()
		if err == nil {
			return ipAddr, nil
		}
		time.Sleep(delay)
	}
	return resolver()
}

func ValidateIpAddresses(ranges []IPRange) error {
	for _, ipRange := range ranges {
		startIP := net.ParseIP(ipRange.Start)
		endIP := net.ParseIP(ipRange.End)
		if startIP == nil {
			return errors.New(fmt.Sprintf("Invalid IP Address for Blacklist IP Range: %s", ipRange.Start))
		}
		if endIP == nil {
			return errors.New(fmt.Sprintf("Invalid IP Address for Blacklist IP Range: %s", ipRange.End))
		}
		if bytes.Compare(startIP, endIP) > 0 {
			return errors.New(fmt.Sprintf("Invalid Blacklist IP Range: Start %s has to be before End %s", ipRange.Start, ipRange.End))
		}
	}
	return nil
}

func IpOutsideOfRanges(testURL url.URL, ranges []IPRange, resolver IPResolver, backoffs ...time.Duration) (bool, error) {
	if len(testURL.Host) == 0 {
		return false, errors.New(fmt.Sprintf("Incomplete URL %s. "+
			"This could be caused by an URL without slashes or protocol.", testURL))
	}

	host := strings.Split(testURL.Host, ":")[0]
	ipAddress := net.ParseIP(host)
	if ipAddress == nil {
		resolvFunc := func() (*net.IPAddr, error) { return resolver.ResolveIPAddr("ip", host) }
		ipAddr, err := retryResolve(resolvFunc, backoffs...)
		if err != nil {
			return false, fmt.Errorf("Resolving host failed: %s", err)
		}
		ipAddress = net.ParseIP(ipAddr.String())
	}

	for _, ipRange := range ranges {
		if bytes.Compare(ipAddress, net.ParseIP(ipRange.Start)) >= 0 && bytes.Compare(ipAddress, net.ParseIP(ipRange.End)) <= 0 {
			return false, nil
		}
	}
	return true, nil
}
