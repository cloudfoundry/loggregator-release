package blacklist

import (
	"doppler/iprange"
	"errors"
	"net"
	"net/url"
	"time"
)

var blacklistBackoffDelays = []time.Duration{
	time.Second,
	2 * time.Second,
	4 * time.Second,
	8 * time.Second,
}

type URLBlacklistManager struct {
	blacklistIPs    []iprange.IPRange
	blacklistedURLs []string
}

func New(blacklistIPs []iprange.IPRange) *URLBlacklistManager {
	return &URLBlacklistManager{blacklistIPs: blacklistIPs}
}

func (blacklistManager *URLBlacklistManager) CheckUrl(rawUrl string) (outputURL *url.URL, err error) {
	outputURL, err = url.Parse(rawUrl)
	if err != nil {
		return nil, err
	}

	resolver := iprange.IPResolverFunc(net.ResolveIPAddr)
	ipNotBlacklisted, err := iprange.IpOutsideOfRanges(*outputURL, blacklistManager.blacklistIPs, resolver, blacklistBackoffDelays...)
	if err != nil {
		return nil, err
	}
	if !ipNotBlacklisted {
		return nil, errors.New("Syslog Drain URL is blacklisted")
	}
	return outputURL, nil
}
