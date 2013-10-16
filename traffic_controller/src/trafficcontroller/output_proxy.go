package trafficcontroller

import (
	"code.google.com/p/go.net/websocket"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"net"
	"net/http"
	"trafficcontroller/authorization"
	"trafficcontroller/hasher"
)

type Proxy struct {
	host      string
	hashers   []*hasher.Hasher
	logger    *gosteno.Logger
	authorize authorization.LogAccessAuthorizer
}

func NewProxy(host string, hashers []*hasher.Hasher, authorizer authorization.LogAccessAuthorizer, logger *gosteno.Logger) *Proxy {
	return &Proxy{host: host, hashers: hashers, authorize: authorizer, logger: logger}
}

func (proxy *Proxy) Start() error {
	return http.ListenAndServe(proxy.host, websocket.Handler(proxy.HandleWebSocket))
}

func (proxy *Proxy) isAuthorized(appId, authToken string, clientAddress net.Addr) int {
	if appId == "" {
		message := fmt.Sprintf("HttpServer: Did not accept sink connection with invalid app id: %s.", clientAddress)
		proxy.logger.Warn(message)
		return 4000
	}

	if authToken == "" {
		message := fmt.Sprintf("HttpServer: Did not accept sink connection from %s without authorization.", clientAddress)
		proxy.logger.Warnf(message)
		return 4001
	}

	if !proxy.authorize(authToken, appId, proxy.logger) {
		message := fmt.Sprintf("HttpServer: Auth token [%s] not authorized to access appId [%s].", authToken, appId)
		proxy.logger.Warn(message)
		return 4002
	}

	return 0
}

func (proxy *Proxy) HandleWebSocket(clientWS *websocket.Conn) {

	req := clientWS.Request()
	req.ParseForm()
	req.Form.Get("app")
	clientAddress := clientWS.RemoteAddr()

	appId := req.Form.Get("app")
	authToken := clientWS.Request().Header.Get("Authorization")

	if code := proxy.isAuthorized(appId, authToken, clientAddress); code > 200 {
		clientWS.CloseWithStatus(code)
		return
	}

	defer clientWS.Close()

	proxy.logger.Debugf("Output Proxy: Request for app: %v", req.Form.Get("app"))
	serverWSs := make([]*websocket.Conn, len(proxy.hashers))
	for index, hasher := range proxy.hashers {
		proxy.logger.Debugf("Output Proxy: Servers in group [%v]: %v", index, hasher.LoggregatorServers())

		server := hasher.GetLoggregatorServerForAppId(appId)
		proxy.logger.Debugf("Output Proxy: AppId is %v. Using server: %v", appId, server)

		config, err := websocket.NewConfig("ws://"+server+req.URL.RequestURI(), "http://localhost")

		if err != nil {
			proxy.logger.Errorf("Output Proxy: Error creating config for websocket - %v", err)
		}

		serverWS, err := websocket.DialConfig(config)
		if err != nil {
			proxy.logger.Errorf("Output Proxy: Error connecting to loggregator server - %v", err)
		}

		if serverWS != nil {
			serverWSs[index] = serverWS
		}
	}
	proxy.forwardIO(serverWSs, clientWS)

}

func (proxy *Proxy) forwardIO(servers []*websocket.Conn, client *websocket.Conn) {
	doneChan := make(chan bool)

	var logMessage []byte
	for _, server := range servers {
		go func(server *websocket.Conn) {
			proxy.logger.Debugf("Output Proxy: Starting to listen to server %v", server.RemoteAddr().String())

			defer server.Close()
			for {
				err := websocket.Message.Receive(server, &logMessage)
				if err != nil {
					proxy.logger.Errorf("Output Proxy: Error reading from the server - %v - %v", err, server.RemoteAddr().String())
					doneChan <- true
					return
				}
				if err == nil {
					proxy.logger.Debugf("Output Proxy: Got message from server %v bytes", len(logMessage))
					websocket.Message.Send(client, logMessage)
				}
			}
		}(server)
	}

	var keepAlive []byte
	go func() {
		for {
			err := websocket.Message.Receive(client, &keepAlive)
			if err != nil {
				proxy.logger.Errorf("Output Proxy: Error reading from the client - %v", err)
				return
			}
			if err == nil {
				proxy.logger.Debugf("Output Proxy: Got message from client %v bytes", len(keepAlive))
				for _, server := range servers {
					websocket.Message.Send(server, keepAlive)
				}
			}
		}
	}()

	for i := 0; i < len(servers); i++ {
		<-doneChan
		proxy.logger.Debug("Output Proxy: Lost one server")
	}
	proxy.logger.Debugf("Output Proxy: Terminating connection. All clients disconnected")
}
