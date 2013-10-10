package trafficcontroller

import (
	"code.google.com/p/go.net/websocket"
	"github.com/cloudfoundry/gosteno"
	"net/http"
	"trafficcontroller/hasher"
)

type Proxy struct {
	host    string
	hashers []*hasher.Hasher
	logger  *gosteno.Logger
}

func NewProxy(host string, hashers []*hasher.Hasher, logger *gosteno.Logger) *Proxy {
	return &Proxy{host: host, hashers: hashers, logger: logger}
}

func (proxy *Proxy) Start() error {
	return http.ListenAndServe(proxy.host, websocket.Handler(proxy.HandleWebSocket))
}

func (proxy *Proxy) HandleWebSocket(clientWS *websocket.Conn) {
	defer clientWS.Close()
	closeChan := make(chan bool)
	req := clientWS.Request()
	req.ParseForm()
	proxy.logger.Debugf("Output Proxy: Request for app: %v", req.Form.Get("app"))
	serverWSs := make([]*websocket.Conn, len(proxy.hashers))
	for index, hasher := range proxy.hashers {
		proxy.logger.Debugf("Output Proxy: Servers in group [%v]: %v", index, hasher.LoggregatorServers())

		server := hasher.GetLoggregatorServerForAppId(req.Form.Get("app"))
		proxy.logger.Debugf("Output Proxy: AppId is %v. Using server: %v", req.Form.Get("app"), server)

		config, err := websocket.NewConfig("ws://"+server+req.URL.RequestURI(), "http://localhost")

		if err != nil {
			proxy.logger.Errorf("Output Proxy: Error creating config for websocket - %v", err)
		}

		authToken := clientWS.Request().Header.Get("Authorization")
		config.Header.Add("Authorization", authToken)

		serverWS, err := websocket.DialConfig(config)
		if err != nil {
			proxy.logger.Errorf("Output Proxy: Error connecting to loggregator server - %v", err)
		}

		if serverWS != nil {
			serverWSs[index] = serverWS
		}
	}
	proxy.forwardIO(serverWSs, clientWS, closeChan)

}

func (proxy *Proxy) forwardIO(servers []*websocket.Conn, client *websocket.Conn, closeChan chan bool) {
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
