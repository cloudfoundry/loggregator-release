package sinks_test

import (
	"io/ioutil"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/cloudfoundry/dropsonde/emitter/fake"
	"github.com/cloudfoundry/dropsonde/metric_sender"
	fakeMS "github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"google.golang.org/grpc/grpclog"
)

var (
	etcdRunner          *etcdstorerunner.ETCDClusterRunner
	pathToTCPEchoServer string
	fakeMetricSender    *fakeMS.FakeMetricSender
	fakeEventEmitter    *fake.FakeEventEmitter
)

func TestSinks(t *testing.T) {
	log.SetOutput(GinkgoWriter)
	grpclog.SetLogger(log.New(ioutil.Discard, "", 0))
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sinks Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	return nil
}, func(encodedBuiltArtifacts []byte) {
	etcdPort := 5000 + (config.GinkgoConfig.ParallelNode)*10
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1, nil)

	etcdRunner.Start()

	var err error
	pathToTCPEchoServer, err = gexec.Build("tools/echo/cmd/tcp_server")
	Expect(err).NotTo(HaveOccurred())

	fakeEventEmitter = fake.NewFakeEventEmitter("doppler")
	fakeMetricSender = fakeMS.NewFakeMetricSender()
	metrics.Initialize(fakeMetricSender, nil)

	sender := metric_sender.NewMetricSender(fakeEventEmitter)
	batcher := metricbatcher.New(sender, 100*time.Millisecond)
	metrics.Initialize(sender, batcher)
}, 10)

var _ = SynchronizedAfterSuite(func() {
	if etcdRunner != nil {
		etcdRunner.Stop()
	}
}, func() {
	gexec.CleanupBuildArtifacts()
})

var _ = BeforeEach(func() {
	etcdRunner.Reset()
})

func AddWSSink(receivedChan chan []byte, port string, path string) (*websocket.Conn, chan bool, <-chan bool) {
	dontKeepAliveChan := make(chan bool, 1)
	connectionDroppedChannel := make(chan bool, 1)

	var ws *websocket.Conn
	var err error
	Eventually(func() error {
		ws, _, err = websocket.DefaultDialer.Dial("ws://127.0.0.1:"+port+path, http.Header{})
		return err
	}).Should(Succeed())
	Expect(ws).NotTo(BeNil())

	go func() {
		for {
			_, data, err := ws.ReadMessage()
			if err != nil {
				connectionDroppedChannel <- true
				close(receivedChan)
				return
			}
			receivedChan <- data
		}
	}()

	go func() {
		for {
			err := ws.WriteMessage(websocket.BinaryMessage, []byte{42})
			if err != nil {
				break
			}
			select {
			case <-dontKeepAliveChan:
				return
			case <-time.After(10 * time.Millisecond):
				// keep-alive
			}
		}
	}()
	return ws, dontKeepAliveChan, connectionDroppedChannel
}
