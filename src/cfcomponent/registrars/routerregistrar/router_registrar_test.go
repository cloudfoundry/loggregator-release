package routerregistrar

import (
	"cfcomponent"
	mbus "github.com/cloudfoundry/go_cfmessagebus"
	"github.com/cloudfoundry/go_cfmessagebus/mock_cfmessagebus"
	"github.com/stretchr/testify/assert"
	"os"
	"testhelpers"
	"testing"
	"time"
)

func TestGreetRouter(t *testing.T) {
	mbus := mock_cfmessagebus.NewMockMessageBus()
	routerReceivedChannel := make(chan []byte)
	fakeRouter(mbus, routerReceivedChannel)

	cfc := &cfcomponent.Component{SystemDomain: "vcap.me", WebPort: 8083}
	registrar := NewRouterRegistrar(mbus, testhelpers.Logger())
	err := registrar.greetRouter(cfc)
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

func TestDefaultIntervalIsSetWhenGreetRouterFails(t *testing.T) {
	mbus := mock_cfmessagebus.NewMockMessageBus()
	cfc := &cfcomponent.Component{SystemDomain: "vcap.me", WebPort: 8083}
	routerReceivedChannel := make(chan []byte)
	fakeBrokenGreeterRouter(mbus, routerReceivedChannel)

	registrar := NewRouterRegistrar(mbus, testhelpers.Logger())
	err := registrar.greetRouter(cfc)
	assert.Error(t, err)

	resultChan := make(chan bool)
	go func() {
		for {
			cfc.RLock()
			if cfc.RegisterInterval == 20*time.Second {
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
		t.Error("Default register interval was never set!")
	}
}

func TestDefaultIntervalIsSetWhenGreetWithoutRouter(t *testing.T) {
	mbus := mock_cfmessagebus.NewMockMessageBus()
	cfc := &cfcomponent.Component{SystemDomain: "vcap.me", WebPort: 8083}
	registrar := NewRouterRegistrar(mbus, testhelpers.Logger())
	err := registrar.greetRouter(cfc)
	assert.Error(t, err)

	resultChan := make(chan bool)
	go func() {
		for {
			cfc.RLock()
			if cfc.RegisterInterval == 20*time.Second {
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
		t.Error("Default register interval was never set!")
	}
}

func TestKeepRegisteringWithRouter(t *testing.T) {
	mbus := mock_cfmessagebus.NewMockMessageBus()
	os.Setenv("LOG_TO_STDOUT", "false")
	routerReceivedChannel := make(chan []byte)
	fakeRouter(mbus, routerReceivedChannel)

	cfc := cfcomponent.Component{SystemDomain: "vcap.me", WebPort: 8083, IpAddress: "13.12.14.15"}
	registrar := NewRouterRegistrar(mbus, testhelpers.Logger())
	cfc.RegisterInterval = 50 * time.Millisecond
	registrar.keepRegisteringWithRouter(cfc)

	for i := 0; i < 3; i++ {
		time.Sleep(55 * time.Millisecond)
		select {
		case msg := <-routerReceivedChannel:
			assert.Equal(t, `registering:{"host":"13.12.14.15","port":8083,"uris":["loggregator.vcap.me"]}`, string(msg))
		default:
			t.Error("Router did not receive a router.register in time!")
		}
	}
}

func TestSubscribeToRouterStart(t *testing.T) {
	mbus := mock_cfmessagebus.NewMockMessageBus()
	cfc := &cfcomponent.Component{SystemDomain: "vcap.me", WebPort: 8083}
	registrar := NewRouterRegistrar(mbus, testhelpers.Logger())
	registrar.subscribeToRouterStart(cfc)

	err := mbus.Publish("router.start", []byte(messageFromRouter))
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

func TestUnregisterFromRouter(t *testing.T) {
	mbus := mock_cfmessagebus.NewMockMessageBus()
	routerReceivedChannel := make(chan []byte)
	fakeRouter(mbus, routerReceivedChannel)

	cfc := cfcomponent.Component{SystemDomain: "vcap.me", WebPort: 8083, IpAddress: "13.12.14.15"}
	registrar := NewRouterRegistrar(mbus, testhelpers.Logger())
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

func fakeRouter(mbus mbus.MessageBus, returnChannel chan []byte) {
	mbus.RespondToChannel("router.greet", func(_ []byte) []byte {
		response := []byte(messageFromRouter)
		return response
	})

	mbus.RespondToChannel("router.register", func(content []byte) []byte {
		returnChannel <- []byte("registering:" + string(content))
		return content
	})

	mbus.RespondToChannel("router.unregister", func(content []byte) []byte {
		returnChannel <- []byte("unregistering:" + string(content))
		return content
	})
}

func fakeBrokenGreeterRouter(mbus mbus.MessageBus, returnChannel chan []byte) {
	mbus.RespondToChannel("router.greet", func(_ []byte) []byte {
		response := []byte("garbel garbel")
		return response
	})
}
