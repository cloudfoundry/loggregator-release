package announcer

import (
	"doppler/config"
	"encoding/json"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/storeadapter"
	"time"
)

type Transport struct {
	Protocol string `json:"protocol"`
	Port     uint32 `json:"port"`
}
type DopplerMeta struct {
	Version    uint32      `json:"version"`
	Ip         string      `json:"ip"`
	Transports []Transport `json:"transports"`
}

const dopplerMetaVersion = 1

func Announce(localIP string, ttl time.Duration, config *config.Config, storeAdapter storeadapter.StoreAdapter, logger *gosteno.Logger) (stopChan chan (chan bool)) {
	if len(config.EtcdUrls) == 0 {
		return
	}

	dopplerMetaBytes, err := buildDopplerMeta(localIP, config)
	if err != nil {
		panic(err)
	}

	key := fmt.Sprintf("/doppler/meta/%s/%s/%d", config.Zone, config.JobName, config.Index)
	logger.Debugf("Starting Health Status Updates to Store: %s", key)

	status, stopChan, err := storeAdapter.MaintainNode(storeadapter.StoreNode{
		Key:   key,
		Value: dopplerMetaBytes,
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

func AnnounceLegacy(localIP string, ttl time.Duration, config *config.Config, storeAdapter storeadapter.StoreAdapter, logger *gosteno.Logger) (stopChan chan (chan bool)) {
	key := fmt.Sprintf("/healthstatus/doppler/%s/%s/%d", config.Zone, config.JobName, config.Index)
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
	dopplerMeta := DopplerMeta{
		Version: dopplerMetaVersion,
		Ip:      localIp,
		Transports: []Transport{
			Transport{Protocol: "udp", Port: config.DropsondeIncomingMessagesPort},
		},
	}

	if config.EnableTLSTransport {
		dopplerMeta.Transports = append(dopplerMeta.Transports, Transport{Protocol: "tls", Port: config.TLSListenerConfig.Port})
	}

	return json.Marshal(dopplerMeta)
}
