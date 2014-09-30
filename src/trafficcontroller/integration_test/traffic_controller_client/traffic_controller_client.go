package traffic_controller_client

import (
	"bytes"
	"crypto/tls"
	gorilla "github.com/gorilla/websocket"
	"net/http"
	"sync"
	"time"
)

type TrafficControllerClient struct {
	tlsConfig *tls.Config
	callback  func()

	ApiEndpoint      string
	receivedMessages [][]byte
	stop             chan struct{}
	done             chan struct{}
	sync.RWMutex
}

func (client *TrafficControllerClient) Start() (error, *http.Response) {

	authToken := "bearer iAmAnAdmin"
	url := client.ApiEndpoint

	header := http.Header{"Origin": []string{"http://localhost"}, "Authorization": []string{authToken}}
	dialer := gorilla.Dialer{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}

	ws, resp, err := dialer.Dial(url, header)

	if err != nil {
		return err, resp
	}

	client.stop = make(chan struct{})
	client.done = make(chan struct{})

	go func() {
		for {
			_, message, err := ws.ReadMessage()

			if err != nil {
				client.done <- struct{}{}
				return
			}

			client.Lock()
			client.receivedMessages = append(client.receivedMessages, message)
			client.Unlock()
		}
	}()

	go func() {
		<-client.stop
		ws.WriteControl(gorilla.CloseMessage, []byte{}, time.Time{})
		ws.Close()
	}()

	return nil, nil
}

func (client *TrafficControllerClient) Stop() {
	if client.stop == nil {
		return
	}
	client.stop <- struct{}{}
	<-client.done
}

func (client *TrafficControllerClient) DidReceiveLegacyMessage(expectedMessage []byte) bool {
	client.Lock()
	defer client.Unlock()
	for _, receivedMessage := range client.receivedMessages {
		if bytes.Compare(expectedMessage, receivedMessage) == 0 {
			return true
		}
	}
	return false
}
