package syslog_drain_store

import (
	"crypto/sha1"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/storeadapter"
	"strings"
	"time"
)

const APP_NODE_TTL = 3600 * time.Second

type SyslogDrainStore interface {
	UpdateDrains(appId string, drainUrls []string) error
	RefreshAppNode(appId string) error
}

type syslogDrainStore struct {
	storeAdapter storeadapter.StoreAdapter
	logger       *gosteno.Logger
}

func NewSyslogDrainStore(storeAdapter storeadapter.StoreAdapter, logger *gosteno.Logger) SyslogDrainStore {
	return &syslogDrainStore{
		storeAdapter: storeAdapter,
		logger:       logger,
	}
}

func (s *syslogDrainStore) UpdateDrains(appId string, drainUrls []string) error {
	var nodes []storeadapter.StoreNode

	drainsToBeDeleted := make(map[string]storeadapter.StoreNode)
	appNode, err := s.storeAdapter.ListRecursively(AppKey(appId))
	if err == nil {
		for _, drainNode := range appNode.ChildNodes {
			drainsToBeDeleted[string(drainNode.Value)] = drainNode
		}
	} else if err != storeadapter.ErrorKeyNotFound {
		return err
	}

	for _, drainUrl := range drainUrls {
		if _, ok := drainsToBeDeleted[drainUrl]; ok {
			s.logger.Debugf("UpdateDrains: skipping drain %v", drainUrl)
			delete(drainsToBeDeleted, drainUrl)
			continue
		}

		if strings.TrimSpace(drainUrl) == "" {
			s.logger.Infof("UpdateDrains: attempted to add whitespace-only drain url '%s' for app %s. Skipping.", drainUrl, appId)
			continue
		}

		s.logger.Debugf("UpdateDrains: adding drain %v", drainUrl)
		node := storeadapter.StoreNode{
			Key:   DrainKey(appId, drainUrl),
			Value: []byte(drainUrl),
		}

		nodes = append(nodes, node)
	}

	for _, drainNode := range drainsToBeDeleted {
		s.logger.Debugf("UpdateDrains: removing drain %v", drainNode.Key)
		err := s.storeAdapter.Delete(drainNode.Key)
		if err != nil {
			return err
		}
	}

	if len(nodes) > 0 {
		err = s.storeAdapter.SetMulti(nodes)
		if err != nil {
			return err
		}

		return s.storeAdapter.UpdateDirTTL(AppKey(appId), uint64(APP_NODE_TTL.Seconds()))
	}

	return nil
}

func (s *syslogDrainStore) RefreshAppNode(appId string) error {
	return s.storeAdapter.UpdateDirTTL(AppKey(appId), uint64(APP_NODE_TTL.Seconds()))
}

func AppKey(appId string) string {
	return fmt.Sprintf("/loggregator/services/%s", appId)
}

func DrainKey(appId, drainUrl string) string {
	hash := sha1.Sum([]byte(drainUrl))
	return fmt.Sprintf("%s/%x", AppKey(appId), hash)
}
