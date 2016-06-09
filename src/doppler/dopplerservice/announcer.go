package dopplerservice

import (
	"doppler/config"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/storeadapter"
)

type DopplerMeta struct {
	Version   uint32   `json:"version"`
	Endpoints []string `json:"endpoints"`
}

const dopplerMetaVersion = 1
const META_ROOT = "/doppler/meta"
const LEGACY_ROOT = "/healthstatus/doppler"

func Announce(localIP string, ttl time.Duration, config *config.Config, storeAdapter storeadapter.StoreAdapter, logger *gosteno.Logger) chan (chan bool) {
	dopplerMetaBytes, err := buildDopplerMeta(localIP, config)
	if err != nil {
		panic(err)
	}

	key := fmt.Sprintf("%s/%s/%s/%s", META_ROOT, config.Zone, config.JobName, config.Index)
	logger.Debugf("Starting Health Status Updates to Store: %s", key)

	node := storeadapter.StoreNode{
		Key:   key,
		Value: dopplerMetaBytes,
		TTL:   uint64(ttl.Seconds()),
	}
	// Call to create to make sure node is created before we return
	storeAdapter.Create(node)
	status, stopChan, err := storeAdapter.MaintainNode(node)

	if err != nil {
		panic(err)
	}

	// The status channel needs to be drained to maintain the node within the etcd cluster
	go func() {
		for stat := range status {
			logger.Debugf("Health updates channel pushed %v at time %v", stat, time.Now())
		}
	}()

	return stopChan
}

func AnnounceLegacy(localIP string, ttl time.Duration, config *config.Config, storeAdapter storeadapter.StoreAdapter, logger *gosteno.Logger) chan (chan bool) {
	key := fmt.Sprintf("%s/%s/%s/%s", LEGACY_ROOT, config.Zone, config.JobName, config.Index)
	status, stopChan, err := storeAdapter.MaintainNode(storeadapter.StoreNode{
		Key:   key,
		Value: []byte(localIP),
		TTL:   uint64(ttl.Seconds()),
	})

	if err != nil {
		panic(err)
	}

	// The status channel needs to be drained to maintain the node within the etcd cluster
	go func() {
		for stat := range status {
			logger.Debugf("Health updates channel pushed %v at time %v", stat, time.Now())
		}
	}()

	return stopChan
}

func buildDopplerMeta(localIp string, config *config.Config) ([]byte, error) {
	udpAddr := fmt.Sprintf("udp://%s:%d", localIp, config.IncomingUDPPort)
	tcpAddr := fmt.Sprintf("tcp://%s:%d", localIp, config.IncomingTCPPort)
	wsAddr := fmt.Sprintf("ws://%s:%d", localIp, config.OutgoingPort)
	dopplerMeta := DopplerMeta{
		Version:   dopplerMetaVersion,
		Endpoints: []string{udpAddr, tcpAddr, wsAddr},
	}

	if config.EnableTLSTransport {
		dopplerMeta.Endpoints = append(dopplerMeta.Endpoints, fmt.Sprintf("tls://%s:%d", localIp, config.TLSListenerConfig.Port))
	}

	return json.Marshal(dopplerMeta)
}
