package elector

import (
	"log"
	"sync/atomic"
	"time"

	"github.com/cloudfoundry/storeadapter"
)

type Elector struct {
	instanceName   []byte
	adapter        storeadapter.StoreAdapter
	updateInterval time.Duration
	isLeader       int32 // 0 = false, 1 = true
}

func NewElector(instanceName string, adapter storeadapter.StoreAdapter, updateInterval time.Duration) *Elector {
	for {
		err := adapter.Connect()

		if err == nil {
			break
		}

		log.Printf("Elector: Unable to connect to store: '%s'", err.Error())
		time.Sleep(updateInterval)
	}

	return &Elector{
		instanceName:   []byte(instanceName),
		adapter:        adapter,
		updateInterval: updateInterval,
	}
}

func (elector *Elector) RunForElection() error {
	var err error

	for {
		err = elector.adapter.Create(elector.generateNode())

		elector.setLeader(err == nil)
		if err == nil { // won election
			log.Printf("Elector: '%s' won election for cluster leader.", elector.instanceName)
			return nil
		}

		cerr, ok := err.(storeadapter.Error)
		if !ok || cerr.Type() != storeadapter.ErrorKeyExists { // weird error with etcd; give up
			log.Printf("Elector: unexpected error from Etcd: %s", err)
			return err
		}

		// lost election
		log.Printf("Elector: '%s' lost election for cluster leader.", elector.instanceName)
		time.Sleep(elector.updateInterval)
	}
}

func (elector *Elector) StayAsLeader() error {
	log.Printf("Elector: '%s' attempting to remain cluster leader…", elector.instanceName)

	node := elector.generateNode()

	err := elector.adapter.CompareAndSwap(node, node)

	elector.setLeader(err == nil)
	return err
}

func (elector *Elector) Vacate() error {
	log.Printf("Elector: '%s' attempting to vacate leadership…", elector.instanceName)

	elector.setLeader(false)
	return elector.adapter.CompareAndDelete(elector.generateNode())
}

func (elector *Elector) IsLeader() bool {
	return atomic.LoadInt32(&elector.isLeader) != 0
}

func (elector *Elector) setLeader(isLeader bool) {
	if isLeader {
		atomic.StoreInt32(&elector.isLeader, 1)
		return
	}

	atomic.StoreInt32(&elector.isLeader, 0)
}

func (elector *Elector) generateNode() storeadapter.StoreNode {
	return storeadapter.StoreNode{
		Key:   "syslog_drain_binder/leader",
		Value: elector.instanceName,
		TTL:   uint64(elector.updateInterval.Seconds()),
	}
}
