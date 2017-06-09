package etcd_syslog_drain_store

import (
	"crypto/sha1"
	"fmt"
	"log"
	"strings"
	"time"

	"code.cloudfoundry.org/loggregator/syslog_drain_binder/shared_types"

	"github.com/cloudfoundry/storeadapter"
)

type EtcdSyslogDrainStore struct {
	storeAdapter storeadapter.StoreAdapter
	ttl          time.Duration
}

func NewEtcdSyslogDrainStore(storeAdapter storeadapter.StoreAdapter, ttl time.Duration) *EtcdSyslogDrainStore {
	return &EtcdSyslogDrainStore{
		storeAdapter: storeAdapter,
		ttl:          ttl,
	}
}

func (store *EtcdSyslogDrainStore) UpdateDrains(allDrainBindings shared_types.AllSyslogDrainBindings) error {

	for appId, drainBinding := range allDrainBindings {
		err := store.updateAppDrains(appId, drainBinding)
		if err != nil {
			return err
		}
	}

	return nil
}

func (store *EtcdSyslogDrainStore) updateAppDrains(appId shared_types.AppID, drainBinding shared_types.SyslogDrainBinding) error {
	var nodes []storeadapter.StoreNode

	for _, drainURL := range drainBinding.DrainURLs {

		if strings.TrimSpace(drainURL) == "" {
			log.Printf("UpdateDrains: attempted to add whitespace-only drain url '%s' for app %s. Skipping.", drainURL, appId)
			continue
		}

		log.Printf("UpdateDrains: adding drain %s to app %s", drainURL, appId)
		drainData := fmt.Sprintf(`{"hostname":"%s","drainURL":"%s"}`, drainBinding.Hostname, drainURL)
		node := storeadapter.StoreNode{
			Key:   drainKey(appId, drainData),
			Value: []byte(drainData),
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

func drainKey(appId shared_types.AppID, drainData string) string {
	hash := sha1.Sum([]byte(drainData))
	return fmt.Sprintf("/loggregator/v2/services/%s/%x", appId, hash)
}
