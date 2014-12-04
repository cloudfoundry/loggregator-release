package elector

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/storeadapter"
	"time"
)

type Elector struct {
	instanceName   []byte
	adapter        storeadapter.StoreAdapter
	updateInterval time.Duration
	logger         *gosteno.Logger
}

func NewElector(instanceName string, adapter storeadapter.StoreAdapter, updateInterval time.Duration, logger *gosteno.Logger) *Elector {

	for {
		err := adapter.Connect()

		if err == nil {
			break
		}

		logger.Errorf("Unable to connect to store: '%s'", err.Error())
		time.Sleep(updateInterval)
	}

	return &Elector{
		instanceName:   []byte(instanceName),
		adapter:        adapter,
		updateInterval: updateInterval,
		logger:         logger,
	}
}

func (elector *Elector) RunForElection() error {
	var err error

	for {
		err = elector.adapter.Create(storeadapter.StoreNode{
			Key:   "syslog_drain_binder/leader",
			Value: elector.instanceName,
			TTL:   uint64(elector.updateInterval.Seconds()),
		})

		if err == nil { // won election
			return nil
		}

		if err != storeadapter.ErrorKeyExists { // weird error with etcd; give up
			return err
		}

		// lost election
		elector.logger.Info("Lost election")
		time.Sleep(elector.updateInterval)
	}
}

func (elector *Elector) StayAsLeader() error {
	node := storeadapter.StoreNode{
		Key:   "syslog_drain_binder/leader",
		Value: elector.instanceName,
		TTL:   uint64(elector.updateInterval.Seconds()),
	}

	return elector.adapter.CompareAndSwap(node, node)
}
