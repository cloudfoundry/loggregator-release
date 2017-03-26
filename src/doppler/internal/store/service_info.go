package store

import (
	"crypto/sha1"
	"fmt"
)

type ServiceInfo struct {
	appId    string
	url      string
	hostname string
}

func NewServiceInfo(appId, url, hostname string) ServiceInfo {
	return ServiceInfo{
		appId:    appId,
		url:      url,
		hostname: hostname,
	}
}

func (s ServiceInfo) Id() string {
	hash := sha1.Sum([]byte(s.url))
	return fmt.Sprintf("%x", hash)
}

func (s ServiceInfo) AppId() string {
	return s.appId
}

func (s ServiceInfo) Url() string {
	return s.url
}

func (s ServiceInfo) Hostname() string {
	return s.hostname
}
