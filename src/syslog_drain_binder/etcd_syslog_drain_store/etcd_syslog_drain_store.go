package etcd_syslog_drain_store

import (
	"crypto/sha1"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/storeadapter"
	"strings"
	"syslog_drain_binder/shared_types"
	"time"
)

const APP_NODE_TTL = 60 * time.Second

type EtcdSyslogDrainStore struct {
	storeAdapter storeadapter.StoreAdapter
	logger       *gosteno.Logger
}

func NewEtcdSyslogDrainStore(storeAdapter storeadapter.StoreAdapter, logger *gosteno.Logger) *EtcdSyslogDrainStore {
	return &EtcdSyslogDrainStore{
		storeAdapter: storeAdapter,
		logger:       logger,
	}
}

func (s *EtcdSyslogDrainStore) UpdateDrains(appDrainUrlMap map[shared_types.AppId][]shared_types.DrainURL) error {

	for appId, drainUrls := range appDrainUrlMap {
		err := s.updateAppDrains(appId, drainUrls)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *EtcdSyslogDrainStore) updateAppDrains(appId shared_types.AppId, drainUrls []shared_types.DrainURL) error {
	var nodes []storeadapter.StoreNode

	for _, drainUrl := range drainUrls {

		if strings.TrimSpace(string(drainUrl)) == "" {
			s.logger.Infof("UpdateDrains: attempted to add whitespace-only drain url '%s' for app %s. Skipping.", drainUrl, appId)
			continue
		}

		s.logger.Debugf("UpdateDrains: adding drain %v", drainUrl)
		node := storeadapter.StoreNode{
			Key:   drainKey(appId, drainUrl),
			Value: []byte(drainUrl),
			TTL:   uint64(APP_NODE_TTL.Seconds()),
		}

		nodes = append(nodes, node)
	}

	if len(nodes) > 0 {
		err := s.storeAdapter.SetMulti(nodes)
		if err != nil {
			return err
		}
	}

	return nil
}

func appKey(appId shared_types.AppId) string {
	return fmt.Sprintf("/loggregator/services/%s", appId)
}

func drainKey(appId shared_types.AppId, drainUrl shared_types.DrainURL) string {
	hash := sha1.Sum([]byte(drainUrl))
	return fmt.Sprintf("%s/%x", appKey(appId), hash)
}
