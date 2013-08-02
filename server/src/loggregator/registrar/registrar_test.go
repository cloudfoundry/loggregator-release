//We're testing only public functions

package registrar

import (
	"cfcomponent"
	"encoding/json"
	mbus "github.com/cloudfoundry/go_cfmessagebus"
	"github.com/cloudfoundry/go_cfmessagebus/mock_cfmessagebus"
	"github.com/cloudfoundry/gosteno"
	"github.com/stretchr/testify/assert"
	"os/exec"
	"testing"
	"time"
)

var mbusClient mbus.MessageBus

func init() {
	_, mbusClient = setupNatsServer(34783)
}

func TestRegisterWithRouter(t *testing.T) {
	routerReceivedChannel := make(chan []byte)
	fakeRouter(mbusClient, routerReceivedChannel)

	cfc := &cfcomponent.Component{SystemDomain: "vcap.me", WebPort: 8083}
	registrar := NewRegistrar(mbusClient, gosteno.NewLogger("TestLogger"))
	err := registrar.RegisterWithRouter(cfc)
	assert.NoError(t, err)

	resultChan := make(chan bool)
	go func() {
		for {
			cfc.RLock()
			if cfc.RegisterInterval == 42*time.Second {
				resultChan <- true
				break
			}
			cfc.RUnlock()
			time.Sleep(5 * time.Millisecond)
		}
		cfc.RUnlock()
	}()

	select {
	case <-resultChan:
	case <-time.After(2 * time.Second):
		t.Error("Router did not receive a router.start in time!")
	}
}

func TestAnnounceComponent(t *testing.T) {
	cfc := &cfcomponent.Component{
		IpAddress:         "1.2.3.4",
		Type:              "Loggregator Server",
		Index:             0,
		StatusPort:        5678,
		StatusCredentials: []string{"user", "pass"},
		UUID:              "abc123",
	}
	mbus := mock_cfmessagebus.NewMockMessageBus()

	called := make(chan []byte)
	callback := func(response []byte) {
		called <- response
	}
	mbus.Subscribe(AnnounceComponentMessageSubject, callback)

	registrar := NewRegistrar(mbus, gosteno.NewLogger("TestLogger"))

	registrar.AnnounceComponent(cfc)

	actual := <-called

	expected := &AnnounceComponentMessage{
		Type:        "Loggregator Server",
		Index:       0,
		Host:        "1.2.3.4:5678",
		UUID:        "0-abc123",
		Credentials: []string{"user", "pass"},
	}

	expectedJson, err := json.Marshal(expected)
	assert.NoError(t, err)
	assert.Equal(t, string(expectedJson), string(actual))
}

func TestSubscribeToComponentDiscover(t *testing.T) {
	cfc := &cfcomponent.Component{
		IpAddress:         "1.2.3.4",
		Type:              "Loggregator Server",
		Index:             0,
		StatusPort:        5678,
		StatusCredentials: []string{"user", "pass"},
		UUID:              "abc123",
	}

	mbus := mock_cfmessagebus.NewMockMessageBus()
	registrar := NewRegistrar(mbus, gosteno.NewLogger("TestLogger"))

	registrar.SubscribeToComponentDiscover(cfc)

	called := make(chan []byte)
	callback := func(response []byte) {
		called <- response
	}

	message := []byte("")
	mbus.Request(DiscoverComponentMessageSubject, message, callback)

	expected := &AnnounceComponentMessage{
		Type:        "Loggregator Server",
		Index:       0,
		Host:        "1.2.3.4:5678",
		UUID:        "0-abc123",
		Credentials: []string{"user", "pass"},
	}

	expectedJson, err := json.Marshal(expected)
	assert.NoError(t, err)
	assert.Equal(t, string(expectedJson), string(<-called))
}

func TestKeepRegisteringWithRouter(t *testing.T) {
	routerReceivedChannel := make(chan []byte)
	fakeRouter(mbusClient, routerReceivedChannel)

	cfc := &cfcomponent.Component{SystemDomain: "vcap.me", WebPort: 8083, IpAddress: "13.12.14.15"}
	registrar := NewRegistrar(mbusClient, gosteno.NewLogger("TestLogger"))
	cfc.RegisterInterval = 50 * time.Millisecond
	registrar.KeepRegisteringWithRouter(cfc)

	for i := 0; i < 3; i++ {
		time.Sleep(55 * time.Millisecond)
		select {
		case msg := <-routerReceivedChannel:
			host := "13.12.14.15"
			assert.Equal(t, `registering:{"host":"`+host+`","port":8083,"uris":["loggregator.vcap.me"]}`, string(msg))
		default:
			t.Error("Router did not receive a router.register in time!")
		}
	}
}

func TestSubscribeToRouterStart(t *testing.T) {
	cfc := &cfcomponent.Component{SystemDomain: "vcap.me", WebPort: 8083}
	registrar := NewRegistrar(mbusClient, gosteno.NewLogger("TestLogger"))
	err := registrar.SubscribeToRouterStart(cfc)
	assert.NoError(t, err)

	err = mbusClient.Publish("router.start", []byte(messageFromRouter))
	assert.NoError(t, err)

	resultChan := make(chan bool)
	go func() {
		for {
			cfc.RLock()
			if cfc.RegisterInterval == 42*time.Second {
				resultChan <- true
				break
			}
			cfc.RUnlock()
			time.Sleep(5 * time.Millisecond)
		}
		cfc.RUnlock()
	}()

	select {
	case <-resultChan:
	case <-time.After(2 * time.Second):
		t.Error("Router did not receive a router.start in time!")
	}
}

func TestUnregister(t *testing.T) {
	routerReceivedChannel := make(chan []byte)
	fakeRouter(mbusClient, routerReceivedChannel)

	cfc := &cfcomponent.Component{SystemDomain: "vcap.me", WebPort: 8083, IpAddress: "13.12.14.15"}
	registrar := NewRegistrar(mbusClient, gosteno.NewLogger("TestLogger"))
	registrar.UnregisterFromRouter(cfc)

	select {
	case msg := <-routerReceivedChannel:
		host := "13.12.14.15"
		assert.Equal(t, `unregistering:{"host":"`+host+`","port":8083,"uris":["loggregator.vcap.me"]}`, string(msg))
	case <-time.After(2 * time.Second):
		t.Error("Router did not receive a router.unregister in time!")
	}
}

const messageFromRouter = `{
  							"id": "some-router-id",
  							"hosts": ["1.2.3.4"],
  							"minimumRegisterIntervalInSeconds": 42
							}`

func fakeRouter(mBusClient mbus.MessageBus, returnChannel chan []byte) {
	mBusClient.RespondToChannel("router.greet", func(_ []byte) []byte {
		response := []byte(messageFromRouter)
		return response
	})

	mBusClient.RespondToChannel("router.register", func(content []byte) []byte {
		returnChannel <- []byte("registering:" + string(content))
		return content
	})

	mBusClient.RespondToChannel("router.unregister", func(content []byte) []byte {
		returnChannel <- []byte("unregistering:" + string(content))
		return content
	})
}

func setupNatsServer(port int) (*exec.Cmd, mbus.MessageBus) {
	natsServerCmd := mbus.StartNats(port)
	mbusClient := newMBusClient(port)
	waitUntilNatsIsUp(mbusClient)
	return natsServerCmd, mbusClient
}

func waitUntilNatsIsUp(mBusClient mbus.MessageBus) {
	natsConnected := make(chan bool, 1)
	go func() {
		for {
			if mBusClient.Publish("asdf", []byte("data")) == nil {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		natsConnected <- true
	}()
	<-natsConnected
	return
}

func newMBusClient(port int) mbus.MessageBus {
	mBusClient, err := mbus.NewMessageBus("NATS")
	if err != nil {
		panic("Could not connect to NATS")
	}
	mBusClient.Configure("127.0.0.1", port, "nats", "nats")
	log := gosteno.NewLogger("TestLogger")
	mBusClient.SetLogger(log)
	for {
		err := mBusClient.Connect()
		if err == nil {
			break
		}
		log.Errorf("Could not connect to NATS: ", err.Error())
		time.Sleep(50 * time.Millisecond)
	}

	return mBusClient
}
