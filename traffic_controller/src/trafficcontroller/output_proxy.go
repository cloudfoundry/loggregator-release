package trafficcontroller

import (
	"code.google.com/p/go.net/websocket"
	"github.com/cloudfoundry/gosteno"
	"net/http"
	"trafficcontroller/hasher"
)

type redirector struct {
	host    string
	hashers []*hasher.Hasher
	logger  *gosteno.Logger
}

func NewProxy(host string, hashers []*hasher.Hasher, logger *gosteno.Logger) *redirector {
	r := &redirector{host: host, hashers: hashers, logger: logger}
	return r
}

func (r *redirector) Start() error {
	return http.ListenAndServe(r.host, websocket.Handler(r.HandleWebSocket))
}

func (r *redirector) HandleWebSocket(clientWS *websocket.Conn) {
	defer clientWS.Close()
	closeChan := make(chan bool)
	req := clientWS.Request()
	req.ParseForm()
	r.logger.Debugf("Request for app: %v", req.Form.Get("app"))
	serverWSs := make([]*websocket.Conn, len(r.hashers))
	for index, hasher := range r.hashers {
		r.logger.Debugf("Hashing between: %v", hasher.LoggregatorServers())

		server, err := hasher.GetLoggregatorServerForAppId(req.Form.Get("app"))
		r.logger.Debugf("Using server: %v", server)

		if err != nil {
			r.logger.Errorf("Error extracting appId from request URL - %v", err)
		}

		config, err := websocket.NewConfig("ws://"+server+req.URL.RequestURI(), "http://localhost")

		if err != nil {
			r.logger.Errorf("Error creating config for websocket - %v", err)
		}

		authToken := clientWS.Request().Header.Get("Authorization")
		config.Header.Add("Authorization", authToken)

		serverWS, err := websocket.DialConfig(config)
		if err != nil {
			r.logger.Errorf("Error connecting to loggregator server - %v", err)
		}

		if serverWS != nil {
			serverWSs[index] = serverWS
		}
	}
	r.forwardIO(serverWSs, clientWS, closeChan)

}

func (r *redirector) forwardIO(servers []*websocket.Conn, client *websocket.Conn, closeChan chan bool) {
	doneChan := make(chan bool)

	var logMessage []byte
	for _, server := range servers {
		go func(server *websocket.Conn) {
			r.logger.Debugf("Starting to listen to server %v", server.RemoteAddr().String())

			defer server.Close()
			for {
				err := websocket.Message.Receive(server, &logMessage)
				if err != nil {
					r.logger.Errorf("Error reading from the server - %v - %v", err, server.RemoteAddr().String())
					doneChan <- true
					return
				}
				if err == nil {
					r.logger.Debugf("Got message from server %v bytes", len(logMessage))
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
				r.logger.Errorf("Error reading from the client - %v", err)
				return
			}
			if err == nil {
				r.logger.Debugf("Got message from client %v bytes", len(keepAlive))
				for _, server := range servers {
					websocket.Message.Send(server, keepAlive)
				}
			}
		}
	}()

	for i := 0; i < len(servers); i++ {
		<-doneChan
		r.logger.Debug("Lost one server")
	}
	r.logger.Debugf("Terminating connection. All clients disconnected")
}
