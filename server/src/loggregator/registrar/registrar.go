package registrar

import (
	"encoding/json"
	"errors"
	mbus "github.com/cloudfoundry/go_cfmessagebus"
	"github.com/cloudfoundry/gosteno"
	"net"
	"strings"
	"sync"
	"time"
)

type CfComponent struct {
	sync.RWMutex
	SystemDomain     string
	WebPort          string
	RegisterInterval time.Duration `json:"minimumRegisterIntervalInSeconds"`
}

const loggregatorHostname = "loggregator"

type registrar struct {
	*gosteno.Logger
	mBusClient mbus.MessageBus
}

func NewRegistrar(mBusClient mbus.MessageBus, logger *gosteno.Logger) *registrar {
	return &registrar{mBusClient: mBusClient, Logger: logger}
}

func (r *registrar) RegisterWithRouter(cfc *CfComponent) (err error) {
	response := make(chan []byte)

	r.mBusClient.Request("router.greet", []byte{}, func(payload []byte) {
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
	r.mBusClient.Subscribe("router.start", func(payload []byte) {
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
			r.publishRouterMessage(cfc, "router.register")
			r.Debug("Reregistered with router")
			<-time.After(cfc.RegisterInterval)
		}
	}()
}

func (r *registrar) Unregister(cfc *CfComponent) {
	r.publishRouterMessage(cfc, "router.unregister")
	r.Info("Unregistered from router")
}

func (r *registrar) publishRouterMessage(cfc *CfComponent, subject string) error {
	host, err := localIP()
	full_hostname := loggregatorHostname + "." + cfc.SystemDomain
	if err != nil {
		r.Warnf("Publishing %s failed, could not look up local IP: %v", subject, err)
	} else {
		message := strings.Join([]string{`{"host":"`, host, `","port":`, cfc.WebPort, `,"uris":["`, full_hostname, `"]}`}, "")
		err := r.mBusClient.Publish(subject, []byte(message))
		if err != nil {
			r.Warnf("Publishing %s failed: %v", subject, err)
		}
	}
	return err
}

func localIP() (string, error) {
	addr, err := net.ResolveUDPAddr("udp", "1.2.3.4:1")
	if err != nil {
		return "", err
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return "", err
	}

	defer conn.Close()

	host, _, err := net.SplitHostPort(conn.LocalAddr().String())
	if err != nil {
		return "", err
	}

	return host, nil
}
