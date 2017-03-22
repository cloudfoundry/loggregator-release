package dopplerservice

import (
	"doppler/config"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/cloudfoundry/storeadapter"
)

type DopplerMeta struct {
	Version   uint32   `json:"version"`
	Endpoints []string `json:"endpoints"`
}

const dopplerMetaVersion = 1
const META_ROOT = "/doppler/meta"
const LEGACY_ROOT = "/healthstatus/doppler"

func Announce(ip string, ttl time.Duration, config *config.Config, storeAdapter storeadapter.StoreAdapter) chan (chan bool) {
	dopplerMetaBytes, err := buildDopplerMeta(ip, config)
	if err != nil {
		panic(err)
	}

	key := fmt.Sprintf("%s/%s/%s/%s", META_ROOT, config.Zone, config.JobName, config.Index)
	log.Printf("Starting Health Status Updates to Store: %s", key)

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
		for range status {
			// Do nothing
		}
	}()

	return stopChan
}

func AnnounceLegacy(ip string, ttl time.Duration, config *config.Config, storeAdapter storeadapter.StoreAdapter) chan (chan bool) {
	key := fmt.Sprintf("%s/%s/%s/%s", LEGACY_ROOT, config.Zone, config.JobName, config.Index)
	status, stopChan, err := storeAdapter.MaintainNode(storeadapter.StoreNode{
		Key:   key,
		Value: []byte(ip),
		TTL:   uint64(ttl.Seconds()),
	})

	if err != nil {
		panic(err)
	}

	// The status channel needs to be drained to maintain the node within the etcd cluster
	go func() {
		for range status {
			// Do nothing
		}
	}()

	return stopChan
}

func buildDopplerMeta(ip string, config *config.Config) ([]byte, error) {
	udpAddr := fmt.Sprintf("udp://%s:%d", ip, config.IncomingUDPPort)
	tcpAddr := fmt.Sprintf("tcp://%s:%d", ip, config.IncomingTCPPort)
	wsAddr := fmt.Sprintf("ws://%s:%d", ip, config.OutgoingPort)
	dopplerMeta := DopplerMeta{
		Version:   dopplerMetaVersion,
		Endpoints: []string{udpAddr, tcpAddr, wsAddr},
	}

	if config.EnableTLSTransport {
		tlsAddr := fmt.Sprintf("tls://%s:%d", ip, config.TLSListenerConfig.Port)
		dopplerMeta.Endpoints = append(dopplerMeta.Endpoints, tlsAddr)
	}

	return json.Marshal(dopplerMeta)
}
