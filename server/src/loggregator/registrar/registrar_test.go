//We're testing only public functions

package registrar

import (
	mbus "github.com/cloudfoundry/go_cfmessagebus"
	"github.com/cloudfoundry/gosteno"
	"github.com/stretchr/testify/assert"
	"os/exec"
	"testing"
	"time"
	"sync"
)

type testSerializer struct {
	sync.RWMutex
}

var ts testSerializer

func init() {
	ts = testSerializer{}
}

func TestRegisterWithRouter(t *testing.T) {
	ts.Lock()
	defer ts.Unlock()

	port := 34783
	routerReceivedChannel := make(chan []byte)

	natsServerCmd, mbusClient := setupNatsServer(port)
	defer mbus.StopNats(natsServerCmd)

	fakeRouter(mbusClient, routerReceivedChannel)

	registrar := NewRegistrar(mbusClient, "vcap.me", "8083", gosteno.NewLogger("TestLogger"))
	err := registrar.RegisterWithRouter()
	assert.NoError(t, err)

	time.Sleep(60 * time.Millisecond)

	registrar.RLock()
	defer registrar.RUnlock()
	assert.Equal(t, registrar.RegisterInterval, 42*time.Second)
}

func TestKeepRegistering(t *testing.T) {
	ts.Lock()
	defer ts.Unlock()

	port := 34784
	routerReceivedChannel := make(chan []byte)

	natsServerCmd, mbusClient := setupNatsServer(port)
	defer mbus.StopNats(natsServerCmd)

	fakeRouter(mbusClient, routerReceivedChannel)

	registrar := NewRegistrar(mbusClient, "vcap.me", "8083", gosteno.NewLogger("TestLogger"))
	registrar.RegisterInterval = 50 * time.Millisecond
	registrar.KeepRegistering()

	for i := 0; i < 3; i++ {
		time.Sleep(55 * time.Millisecond)
		select {
		case msg := <-routerReceivedChannel:
			host, err := localIP()
			assert.NoError(t, err)
			assert.Equal(t, `registering:{"host":"`+host+`","port":8083,"uris":["loggregator.vcap.me"]}`, string(msg))
		default:
			t.Error("Router did not receive a router.register in time!")
		}
	}
}

func TestSubscribeToRouterStart(t *testing.T) {
	ts.Lock()
	defer ts.Unlock()

	port := 34785
	natsServerCmd, mbusClient := setupNatsServer(port)
	defer mbus.StopNats(natsServerCmd)

	registrar := NewRegistrar(mbusClient, "vcap.me", "8083", gosteno.NewLogger("TestLogger"))
	err := registrar.SubscribeToRouterStart()
	assert.NoError(t, err)

	err = mbusClient.Publish("router.start", []byte(messageFromRouter))
	assert.NoError(t, err)

	time.Sleep(60 * time.Millisecond)

	registrar.RLock()
	defer registrar.RUnlock()
	assert.Equal(t, registrar.RegisterInterval, 42*time.Second)
}

func TestUnregister(t *testing.T) {
	ts.Lock()
	defer ts.Unlock()

	port := 34786
	routerReceivedChannel := make(chan []byte)

	natsServerCmd, mbusClient := setupNatsServer(port)
	defer mbus.StopNats(natsServerCmd)

	fakeRouter(mbusClient, routerReceivedChannel)

	registrar := NewRegistrar(mbusClient, "vcap.me", "8083", gosteno.NewLogger("TestLogger"))
	registrar.Unregister()

	select {
	case msg := <-routerReceivedChannel:
		host, err := localIP()
		assert.NoError(t, err)
		assert.Equal(t, `unregistering:{"host":"`+host+`","port":8083,"uris":["loggregator.vcap.me"]}`, string(msg))
	case <-time.After(50 * time.Millisecond):
		t.Error("Router did not receive a router.unregister in time!")
	}
}

const messageFromRouter = `{
  							"id": "some-router-id",
  							"hosts": ["1.2.3.4"],
  							"minimumRegisterIntervalInSeconds": 42
							}`

func fakeRouter(mBusClient mbus.CFMessageBus, returnChannel chan []byte) {
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

func setupNatsServer(port int) (*exec.Cmd, mbus.CFMessageBus) {
	natsServerCmd := mbus.StartNats(port)
	mbusClient := newMBusClient(port)
	<-waitUntilNatsIsUp(mbusClient)
	return natsServerCmd, mbusClient
}

func waitUntilNatsIsUp(mBusClient mbus.CFMessageBus) chan bool {
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
	return natsConnected
}

func newMBusClient(port int) mbus.CFMessageBus {
	mBusClient, err := mbus.NewCFMessageBus("NATS")
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
