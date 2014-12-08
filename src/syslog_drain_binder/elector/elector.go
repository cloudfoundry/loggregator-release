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
		err = elector.adapter.Create(elector.generateNode())

		elector.isLeader = (err == nil)
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
	node := elector.generateNode()

	err := elector.adapter.CompareAndSwap(node, node)

	elector.isLeader = (err == nil)
	return err
}

func (elector *Elector) Vacate() error {
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
