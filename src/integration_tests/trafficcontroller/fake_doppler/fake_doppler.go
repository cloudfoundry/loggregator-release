package fake_doppler

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/server/handlers"
	gorilla "github.com/gorilla/websocket"
)

type FakeDoppler struct {
	ApiEndpoint                string
	connectionListener         net.Listener
	websocket                  *gorilla.Conn
	sendMessageChan            chan []byte
	Ready                      chan struct{}
	TrafficControllerConnected chan *http.Request
	connectionPresent          bool
	sync.RWMutex
}

func (fakeDoppler *FakeDoppler) Start() {
	fakeDoppler.Lock()
	defer fakeDoppler.Unlock()

	connectionListener, e := net.Listen("tcp", fakeDoppler.ApiEndpoint)
	if e != nil {
		panic(e)
	}

	fakeDoppler.connectionListener = connectionListener
	fakeDoppler.ResetMessageChan()
	fakeDoppler.Ready = make(chan struct{})
	fakeDoppler.TrafficControllerConnected = make(chan *http.Request, 1)

	go func() {
		s := &http.Server{Addr: fakeDoppler.ApiEndpoint, Handler: fakeDoppler}
		fakeDoppler.Ready <- struct{}{}
		s.Serve(fakeDoppler.connectionListener)
	}()
}

func (fakeDoppler *FakeDoppler) Stop() {
	fakeDoppler.Lock()
	defer fakeDoppler.Unlock()
	fakeDoppler.connectionListener.Close()
}

func (fakeDoppler *FakeDoppler) ResetMessageChan() {
	fakeDoppler.sendMessageChan = make(chan []byte, 100)
}

func (fakeDoppler *FakeDoppler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	select {
	case fakeDoppler.TrafficControllerConnected <- request:
	default:
	}

	fakeDoppler.Lock()
	fakeDoppler.connectionPresent = true
	fakeDoppler.Unlock()

	handlers.NewWebsocketHandler(fakeDoppler.sendMessageChan, time.Millisecond*100, loggertesthelper.Logger()).ServeHTTP(writer, request)

	fakeDoppler.Lock()
	fakeDoppler.connectionPresent = false
	fakeDoppler.Unlock()
}

func (fakeDoppler *FakeDoppler) ConnectionPresent() bool {
	fakeDoppler.Lock()
	defer fakeDoppler.Unlock()

	return fakeDoppler.connectionPresent
}

func (fakeDoppler *FakeDoppler) SendLogMessage(messageBody []byte) {
	fakeDoppler.sendMessageChan <- messageBody
}

func (fakeDoppler *FakeDoppler) CloseLogMessageStream() {
	close(fakeDoppler.sendMessageChan)
}
