package registrar

import (
	"encoding/json"
	"errors"
	"fmt"
	mbus "github.com/cloudfoundry/go_cfmessagebus"
	"github.com/cloudfoundry/gosteno"
	"sync"
	"time"
)

type CfComponent struct {
	sync.RWMutex
	IpAddress         string
	SystemDomain      string
	WebPort           uint32
	RegisterInterval  time.Duration `json:"minimumRegisterIntervalInSeconds"`
	Type              string        //Used by the collector to find data processing class
	Index             uint
	UUID              string
	StatusPort        uint32
	StatusCredentials []string
}

const loggregatorHostname = "loggregator"

type registrar struct {
	*gosteno.Logger
	mBusClient mbus.MessageBus
}

func NewRegistrar(mBusClient mbus.MessageBus, logger *gosteno.Logger) *registrar {
	return &registrar{mBusClient: mBusClient, Logger: logger}
}

func (r *registrar) AnnounceComponent(cfc *CfComponent) error {
	json, err := json.Marshal(NewAnnounceComponentMessage(cfc))
	if err != nil {
		return err
	}

	r.mBusClient.Publish(AnnounceComponentMessageSubject, json)
	return nil
}

func (r *registrar) SubscribeToComponentDiscover(cfc *CfComponent) error {
	r.mBusClient.RespondToChannel(DiscoverComponentMessageSubject, func(msg []byte) []byte {
		json, err := json.Marshal(NewAnnounceComponentMessage(cfc))
		if err != nil {
			r.Warnf("Failed to marshal response to message [%s]: %s", DiscoverComponentMessageSubject, err.Error())
			return nil
		}
		return json
	})
	return nil
}

func (r *registrar) RegisterWithRouter(cfc *CfComponent) (err error) {
	response := make(chan []byte)

	r.mBusClient.Request(RouterGreetMessageSubject, []byte{}, func(payload []byte) {
		response <- payload
	})

	select {
	case msg := <-response:
		cfc.Lock()
		defer cfc.Unlock()
		err = json.Unmarshal(msg, cfc)
		if err != nil {
			r.Errorf("Error unmarshalling the greet response: %v\n", err)
		} else {
			r.Infof("Greeted the router. Setting register interval to %v seconds\n", cfc.RegisterInterval)
			cfc.RegisterInterval = cfc.RegisterInterval * time.Second
		}
	case <-time.After(2 * time.Second):
		err = errors.New("Did not get a response to router.greet!")
	}

	return err
}

func (r *registrar) SubscribeToRouterStart(cfc *CfComponent) (err error) {
	r.mBusClient.Subscribe(RouterStartMessageSubject, func(payload []byte) {
		cfc.Lock()
		defer cfc.Unlock()
		err = json.Unmarshal(payload, cfc)
		if err != nil {
			r.Errorf("Error unmarshalling the router start message: %v\n", err)
		} else {
			r.Infof("Received router.start. Setting register interval to %v seconds\n", cfc.RegisterInterval)
			cfc.RegisterInterval = cfc.RegisterInterval * time.Second
		}
	})
	r.Info("Subscribed to router.start")

	return err
}

func (r *registrar) KeepRegisteringWithRouter(cfc *CfComponent) {
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

func (r *registrar) UnregisterFromRouter(cfc *CfComponent) {
	err := r.publishRouterMessage(cfc, RouterUnregisterMessageSubject)
	if err != nil {
		r.Error(err.Error())
	}
	r.Info("Unregistered from router")
}

func (r *registrar) publishRouterMessage(cfc *CfComponent, subject string) error {
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
