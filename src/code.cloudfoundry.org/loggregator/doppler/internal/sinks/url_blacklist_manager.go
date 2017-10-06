package sinks

import (
	"errors"
	"log"
	"net/url"
)

type URLBlacklistManager struct {
	blacklistIPs    []IPRange
	blacklistedURLs []string
}

func NewBlackListManager(blacklistIPs []IPRange) *URLBlacklistManager {
	return &URLBlacklistManager{blacklistIPs: blacklistIPs}
}

func (blacklistManager *URLBlacklistManager) CheckUrl(rawUrl string) (outputURL *url.URL, err error) {
	outputURL, err = url.Parse(rawUrl)
	if err != nil {
		return nil, err
	}

	ipNotBlacklisted, err := IpOutsideOfRanges(*outputURL, blacklistManager.blacklistIPs)
	if err != nil {
		_, ok := err.(ResolutionFailure)
		if !ok {
			return nil, err
		}
		log.Printf("Could not resolve URL %s: %s", outputURL, err)

		// Allow the syslog drain backoff to deal with this URL if it
		// continues to not resolve.
		ipNotBlacklisted = true
	}
	if !ipNotBlacklisted {
		return nil, errors.New("Syslog Drain URL is blacklisted")
	}
	return outputURL, nil
}
