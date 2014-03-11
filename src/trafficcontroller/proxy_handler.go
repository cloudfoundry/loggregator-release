package trafficcontroller

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/gorilla/websocket"
	"net/http"
	"sync"
	"time"
	"trafficcontroller/hasher"
)

type handler struct {
	sync.Mutex
	sync.WaitGroup
	clientWS clientWebsocket
	logger   *gosteno.Logger
}

type clientWebsocket interface {
	WriteMessage(int, []byte) error
	ReadMessage() (messageType int, p []byte, err error)
	WriteControl(int, []byte, time.Time) error
}

func NewProxyHandler(clientWS clientWebsocket, logger *gosteno.Logger) *handler {
	return &handler{clientWS: clientWS, logger: logger}
}

func (handler *handler) HandleWebSocket(appId, requestUri string, hashers []*hasher.Hasher) {
	defer handler.clientWS.WriteControl(websocket.CloseMessage, []byte{}, time.Time{})

	handler.logger.Debugf("Ouxtput Proxy: Request for app: %v", appId)
	serverWSs := make([]*websocket.Conn, len(hashers))

	for index, hasher := range hashers {
		handler.logger.Debugf("Output Proxy: Servers in group [%v]: %v", index, hasher.LoggregatorServers())

		server := hasher.GetLoggregatorServerForAppId(appId)
		handler.logger.Debugf("Output Proxy: AppId is %v. Using server: %v", appId, server)

		serverWS, _, err := websocket.DefaultDialer.Dial("ws://"+server+requestUri, http.Header{})

		if err != nil {
			handler.logger.Errorf("Output Proxy: Error connecting to loggregator server - %v", err)
		} else if serverWS != nil {
			serverWSs[index] = serverWS
		}
	}
	handler.forwardIO(serverWSs)

}

func (handler *handler) proxyConnectionTo(server *websocket.Conn) {
	handler.logger.Debugf("Output Proxy: Starting to listen to server %v", server.RemoteAddr().String())

	var logMessage []byte
	defer server.Close()
	defer handler.Done()
	count := 0
	for {
		_, data, err := server.ReadMessage()

		if err != nil {
			handler.logger.Errorf("Output Proxy: Error reading from the server - %v - %v", err, server.RemoteAddr().String())
			return
		}

		handler.logger.Debugf("Output Proxy: Got message from server %v bytes", len(logMessage))

		count++

		err = handler.writeMessage(data)

		if err != nil {
			handler.logger.Errorf("Output Proxy: Error writing to client websocket - %v", err)
			return
		}
	}
}

func (handler *handler) writeMessage(data []byte) error {
	handler.Lock()
	defer handler.Unlock()
	return handler.clientWS.WriteMessage(websocket.BinaryMessage, data)
}

func (handler *handler) watchKeepAlive(servers []*websocket.Conn) {
	for {
		_, keepAlive, err := handler.clientWS.ReadMessage()

		if err != nil {
			handler.logger.Errorf("Output Proxy: Error reading from the client - %v", err)
			for _, server := range servers {
				server.WriteControl(websocket.CloseMessage, []byte{}, time.Time{})
			}

			return
		}
		handler.logger.Debugf("Output Proxy: Got message from client %v bytes", len(keepAlive))
		for _, server := range servers {
			server.WriteMessage(websocket.BinaryMessage, keepAlive)
		}
	}
}

func (handler *handler) forwardIO(servers []*websocket.Conn) {
	handler.WaitGroup.Add(len(servers))
	for _, server := range servers {
		go handler.proxyConnectionTo(server)
	}
	go handler.watchKeepAlive(servers)

	handler.Wait()

	handler.logger.Debugf("Output Proxy: Terminating connection. All clients disconnected")
}
