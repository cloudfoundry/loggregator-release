package collectorregistrar

import (
	"cfcomponent"
	"encoding/json"
	mbus "github.com/cloudfoundry/go_cfmessagebus"
	"github.com/cloudfoundry/gosteno"
)

type collectorRegistrar struct {
	*gosteno.Logger
	mBusClient mbus.MessageBus
}

func NewCollectorRegistrar(mBusClient mbus.MessageBus, logger *gosteno.Logger) *collectorRegistrar {
	return &collectorRegistrar{mBusClient: mBusClient, Logger: logger}
}

func (r *collectorRegistrar) RegisterWithCollector(cfc cfcomponent.Component) error {
	err := r.announceComponent(cfc)
	r.subscribeToComponentDiscover(cfc)

	return err
}

func (r *collectorRegistrar) announceComponent(cfc cfcomponent.Component) error {
	json, err := json.Marshal(NewAnnounceComponentMessage(cfc))
	if err != nil {
		return err
	}

	r.mBusClient.Publish(AnnounceComponentMessageSubject, json)
	return nil
}

func (r *collectorRegistrar) subscribeToComponentDiscover(cfc cfcomponent.Component) {
	r.mBusClient.RespondToChannel(DiscoverComponentMessageSubject, func(msg []byte) []byte {
		json, err := json.Marshal(NewAnnounceComponentMessage(cfc))
		if err != nil {
			r.Warnf("Failed to marshal response to message [%s]: %s", DiscoverComponentMessageSubject, err.Error())
			return nil
		}
		return json
	})
	return
}
