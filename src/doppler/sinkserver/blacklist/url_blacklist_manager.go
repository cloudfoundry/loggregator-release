package blacklist

import (
	"doppler/iprange"
	"errors"
	"net/url"

	"github.com/cloudfoundry/gosteno"
)

type URLBlacklistManager struct {
	blacklistIPs    []iprange.IPRange
	blacklistedURLs []string
	logger          *gosteno.Logger
}

func New(blacklistIPs []iprange.IPRange, logger *gosteno.Logger) *URLBlacklistManager {
	return &URLBlacklistManager{blacklistIPs: blacklistIPs, logger: logger}
}

func (blacklistManager *URLBlacklistManager) CheckUrl(rawUrl string) (outputURL *url.URL, err error) {
	outputURL, err = url.Parse(rawUrl)
	if err != nil {
		return nil, err
	}

	ipNotBlacklisted, err := iprange.IpOutsideOfRanges(*outputURL, blacklistManager.blacklistIPs)
	if err != nil {
		_, ok := err.(iprange.ResolutionFailure)
		if !ok {
			return nil, err
		}
		blacklistManager.logger.Errorf("Could not resolve URL %s: %s", outputURL, err)

		// Allow the syslog drain backoff to deal with this URL if it
		// continues to not resolve.
		ipNotBlacklisted = true
	}
	if !ipNotBlacklisted {
		return nil, errors.New("Syslog Drain URL is blacklisted")
	}
	return outputURL, nil
}
