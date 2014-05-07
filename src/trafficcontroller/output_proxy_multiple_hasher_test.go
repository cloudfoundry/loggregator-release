package trafficcontroller_test

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"sync"
	"trafficcontroller"
	"trafficcontroller/hasher"
	"trafficcontroller/listener"
	testhelpers "trafficcontroller_testhelpers"
)

type fakeHasher struct {
	sync.Mutex
	hostAndPort  string
	proxyStarted bool
}

func (f *fakeHasher) LoggregatorServers() []string {
	return []string{f.hostAndPort}
}

func (f *fakeHasher) GetLoggregatorServerForAppId(string) string {
	return f.hostAndPort
}
func (f *fakeHasher) ProxyMessagesFor(string, string, listener.OutputChannel, listener.StopChannel) error {
	f.Lock()
	defer f.Unlock()
	f.proxyStarted = true
	return nil
}

func (f *fakeHasher) isStarted() bool {
	f.Lock()
	defer f.Unlock()
	return f.proxyStarted
}

var _ = Describe("OutputProxyMultipleHasher", func() {

	var fwsh *fakeWebsocketHandler
	var fakehashers []*fakeHasher
	var PORT = "62028"
	var proxy *trafficcontroller.Proxy

	BeforeEach(func() {
		fwsh = &fakeWebsocketHandler{}
		trafficcontroller.NewWebsocketHandlerProvider = func(messageChan <-chan []byte, logger *gosteno.Logger) http.Handler {
			return fwsh
		}

		fakehashers = []*fakeHasher{
			&fakeHasher{hostAndPort: "localhost:62038"},
			&fakeHasher{hostAndPort: "localhost:62039"},
		}

		hashers := []hasher.Hasher{
			fakehashers[0],
			fakehashers[1],
		}

		proxy = trafficcontroller.NewProxy(
			"localhost:"+PORT,
			hashers,
			testhelpers.SuccessfulAuthorizer,
			loggertesthelper.Logger(),
		)
		go proxy.Start()
	})

	AfterEach(func() {
		proxy.Stop()
	})

	It("should listen to all hashers", func() {
		websocketClientWithHeaderAuth(PORT, "/?app=myApp", testhelpers.VALID_AUTHENTICATION_TOKEN)
		Expect(fakehashers[0].isStarted()).To(BeTrue())
		Expect(fakehashers[1].isStarted()).To(BeTrue())
	})
})
