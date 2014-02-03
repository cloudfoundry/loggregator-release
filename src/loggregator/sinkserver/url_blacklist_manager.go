package sinkserver

import (
	"errors"
	"loggregator/iprange"
	"net/url"
)

type URLBlacklistManager struct {
	blacklistIPs    []iprange.IPRange
	blacklistedURLs []string
}

func (blacklistManager *URLBlacklistManager) isBlackListed(testUrl string) bool {
	for _, url := range blacklistManager.blacklistedURLs {
		if url == testUrl {
			return true
		}
	}
	return false
}

func (blacklistManager *URLBlacklistManager) checkURL(rawUrl string) (outputURL *url.URL, err error) {
	outputURL, err = url.Parse(rawUrl)
	if err != nil {
		return nil, err
	}

	ipNotBlacklisted, err := iprange.IpOutsideOfRanges(*outputURL, blacklistManager.blacklistIPs)
	if err != nil {
		return nil, err
	}
	if !ipNotBlacklisted {
		return nil, errors.New("Syslog Drain URL is blacklisted")
	}
	return outputURL, nil
}

func (blacklistManager *URLBlacklistManager) blacklistURL(url string) {
	blacklistManager.blacklistedURLs = append(blacklistManager.blacklistedURLs, url)
}
