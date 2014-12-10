package etcd_syslog_drain_store

import (
	"crypto/sha1"
	"fmt"
	"strings"
	"time"

	"syslog_drain_binder/shared_types"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/storeadapter"
)

type EtcdSyslogDrainStore struct {
	storeAdapter storeadapter.StoreAdapter
	ttl          time.Duration
	logger       *gosteno.Logger
}

func NewEtcdSyslogDrainStore(storeAdapter storeadapter.StoreAdapter, ttl time.Duration, logger *gosteno.Logger) *EtcdSyslogDrainStore {
	return &EtcdSyslogDrainStore{
		storeAdapter: storeAdapter,
		ttl:          ttl,
		logger:       logger,
	}
}

func (store *EtcdSyslogDrainStore) UpdateDrains(appDrainUrlMap map[shared_types.AppId][]shared_types.DrainURL) error {

	for appId, drainUrls := range appDrainUrlMap {
		err := store.updateAppDrains(appId, drainUrls)
		if err != nil {
			return err
		}
	}

	return nil
}

func (store *EtcdSyslogDrainStore) updateAppDrains(appId shared_types.AppId, drainUrls []shared_types.DrainURL) error {
	var nodes []storeadapter.StoreNode

	for _, drainUrl := range drainUrls {

		if strings.TrimSpace(string(drainUrl)) == "" {
			store.logger.Infof("UpdateDrains: attempted to add whitespace-only drain url '%s' for app %s. Skipping.", drainUrl, appId)
			continue
		}

		store.logger.Debugf("UpdateDrains: adding drain %s to app %s", drainUrl, appId)
		node := storeadapter.StoreNode{
			Key:   drainKey(appId, drainUrl),
			Value: []byte(drainUrl),
			TTL:   uint64(store.ttl.Seconds()),
		}

		nodes = append(nodes, node)
	}

	if len(nodes) > 0 {
		err := store.storeAdapter.SetMulti(nodes)
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
