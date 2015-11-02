package helpers

import (
	"crypto/sha1"
	"fmt"
	"github.com/cloudfoundry/storeadapter"
)

type SyslogRegistrar struct {
	etcdStoreAdapter storeadapter.StoreAdapter
}

func NewSyslogRegistrar(etcdStoreAdapter storeadapter.StoreAdapter) *SyslogRegistrar {
	return &SyslogRegistrar{
		etcdStoreAdapter: etcdStoreAdapter,
	}
}

func (s *SyslogRegistrar) Register(appID string, syslogURL string) {
	node := storeadapter.StoreNode{
		Key:   drainKey(appID, syslogURL),
		Value: []byte(syslogURL),
	}

	err := s.etcdStoreAdapter.Create(node)
	if err != nil {
		panic(err)
	}

}

func (s *SyslogRegistrar) UnRegister(appID string, syslogURL string) {
	key := drainKey(appID, syslogURL)
	s.etcdStoreAdapter.Delete(key)
}

func appKey(appId string) string {
	return fmt.Sprintf("/loggregator/services/%s", appId)
}

func drainKey(appId string, drainUrl string) string {
	hash := sha1.Sum([]byte(drainUrl))
	return fmt.Sprintf("%s/%x", appKey(appId), hash)
}
