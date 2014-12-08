package elector

import (
	"time"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/storeadapter"
)

type Elector struct {
	instanceName   []byte
	adapter        storeadapter.StoreAdapter
	updateInterval time.Duration
	isLeader       bool
	logger         *gosteno.Logger
}

func NewElector(instanceName string, adapter storeadapter.StoreAdapter, updateInterval time.Duration, logger *gosteno.Logger) *Elector {
	for {
		err := adapter.Connect()

		if err == nil {
			break
		}

		logger.Errorf("Elector: Unable to connect to store: '%s'", err.Error())
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
		err = elector.adapter.Create(elector.generateNode())

		elector.isLeader = (err == nil)
		if err == nil { // won election
			elector.logger.Infof("Elector: '%s' won election for cluster leader.", elector.instanceName)
			return nil
		}

		if err != storeadapter.ErrorKeyExists { // weird error with etcd; give up
			elector.logger.Errorf("Elector: unexpected error from Etcd: %s", err)
			return err
		}

		// lost election
		elector.logger.Infof("Elector: '%s' lost election for cluster leader.", elector.instanceName)
		time.Sleep(elector.updateInterval)
	}
}

func (elector *Elector) StayAsLeader() error {
	elector.logger.Debugf("Elector: '%s' attempting to remain cluster leader…", elector.instanceName)

	node := elector.generateNode()

	err := elector.adapter.CompareAndSwap(node, node)

	elector.isLeader = (err == nil)
	return err
}

func (elector *Elector) Vacate() error {
	elector.logger.Debugf("Elector: '%s' attempting to vacate leadership…", elector.instanceName)

	elector.isLeader = false
	return elector.adapter.CompareAndDelete(elector.generateNode())
}

func (elector Elector) IsLeader() bool {
	return elector.isLeader
}

func (elector *Elector) generateNode() storeadapter.StoreNode {
	return storeadapter.StoreNode{
		Key:   "syslog_drain_binder/leader",
		Value: elector.instanceName,
		TTL:   uint64(elector.updateInterval.Seconds()),
	}
}
