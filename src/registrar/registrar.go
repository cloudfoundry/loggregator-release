package registrar

import (
	"cfcomponent"
	"encoding/json"
	"errors"
	"fmt"
	mbus "github.com/cloudfoundry/go_cfmessagebus"
	"github.com/cloudfoundry/gosteno"
	"time"
)

const loggregatorHostname = "loggregator"

type registrar struct {
	*gosteno.Logger
	mBusClient mbus.MessageBus
}

func NewRegistrar(mBusClient mbus.MessageBus, logger *gosteno.Logger) *registrar {
	return &registrar{mBusClient: mBusClient, Logger: logger}
}

func (r *registrar) RegisterWithRouter(cfc *cfcomponent.Component) error {
	r.subscribeToRouterStart(cfc)
	err := r.greetRouter(cfc)
	if err != nil {
		return err
	}
	r.keepRegisteringWithRouter(*cfc)

	return nil
}

func (r *registrar) RegisterWithCollector(cfc cfcomponent.Component) (err error) {
	err = r.announceComponent(cfc)
	r.subscribeToComponentDiscover(cfc)

	return
}

func (r *registrar) announceComponent(cfc cfcomponent.Component) error {
	json, err := json.Marshal(NewAnnounceComponentMessage(cfc))
	if err != nil {
		return err
	}

	r.mBusClient.Publish(AnnounceComponentMessageSubject, json)
	return nil
}

func (r *registrar) subscribeToComponentDiscover(cfc cfcomponent.Component) {
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

func (r *registrar) greetRouter(cfc *cfcomponent.Component) (err error) {
	response := make(chan []byte)

	r.mBusClient.Request(RouterGreetMessageSubject, []byte{}, func(payload []byte) {
		response <- payload
	})

	select {
	case msg := <-response:
		routerResponse := &RouterResponse{}
		err = json.Unmarshal(msg, routerResponse)
		if err != nil {
			r.Errorf("Error unmarshalling the greet response: %v\n", err)
		} else {
			r.Infof("Greeted the router. Setting register interval to %v seconds\n", routerResponse.RegisterInterval)
			cfc.Lock()
			cfc.RegisterInterval = routerResponse.RegisterInterval * time.Second
			cfc.Unlock()

		}
	case <-time.After(2 * time.Second):
		err = errors.New("Did not get a response to router.greet!")
	}

	return err
}

func (r *registrar) subscribeToRouterStart(cfc *cfcomponent.Component) {
	r.mBusClient.Subscribe(RouterStartMessageSubject, func(payload []byte) {
		routerResponse := &RouterResponse{}
		err := json.Unmarshal(payload, routerResponse)
		if err != nil {
			r.Errorf("Error unmarshalling the router start message: %v\n", err)
		} else {
			r.Infof("Received router.start. Setting register interval to %v seconds\n", cfc.RegisterInterval)
			cfc.Lock()
			cfc.RegisterInterval = routerResponse.RegisterInterval * time.Second
			cfc.Unlock()
		}
	})
	r.Info("Subscribed to router.start")

	return
}

func (r *registrar) keepRegisteringWithRouter(cfc cfcomponent.Component) {
	go func() {
		for {
			err := r.publishRouterMessage(cfc, RouterRegisterMessageSubject)
			if err != nil {
				r.Error(err.Error())
			}
			r.Debug("Reregistered with router")
			<-time.After(cfc.RegisterInterval)
		}
	}()
}

func (r *registrar) UnregisterFromRouter(cfc cfcomponent.Component) {
	err := r.publishRouterMessage(cfc, RouterUnregisterMessageSubject)
	if err != nil {
		r.Error(err.Error())
	}
	r.Info("Unregistered from router")
}

func (r *registrar) publishRouterMessage(cfc cfcomponent.Component, subject string) error {
	full_hostname := loggregatorHostname + "." + cfc.SystemDomain
	message := &RouterMessage{
		Host: cfc.IpAddress,
		Port: cfc.WebPort,
		Uris: []string{full_hostname},
	}

	json, err := json.Marshal(message)
	if err != nil {
		return errors.New(fmt.Sprintf("Error marshalling the router message: %v\n", err))
	}

	err = r.mBusClient.Publish(subject, json)
	if err != nil {
		return errors.New(fmt.Sprintf("Publishing %s failed: %v", subject, err))
	}
	return nil
}
