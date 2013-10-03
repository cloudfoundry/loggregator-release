package trafficcontroller

import (
	"code.google.com/p/go.net/websocket"
	testhelpers "github.com/cloudfoundry/loggregatorlib/lib_testhelpers"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
	"time"
	"trafficcontroller/hasher"
)

func TestProxying(t *testing.T) {
	go Server("localhost:62023", "Hello World from the server", 1)

	proxy := NewProxy(
		"localhost:62022",
		[]*hasher.Hasher{hasher.NewHasher([]string{"localhost:62023"})},
		testhelpers.Logger(),
	)
	go proxy.Start()
	WaitForServerStart("62023", "/")

	receivedChan := Client(t, "62022", "/?app=myApp")

	select {
	case data := <-receivedChan:
		assert.Equal(t, string(data), "Hello World from the server")
	case <-time.After(1 * time.Second):
		t.Error("Did not receive response within one second")
	}
}

func TestProxyingWithTwoMessages(t *testing.T) {
	go Server("localhost:62020", "Hello World from the server", 2)

	proxy := NewProxy(
		"localhost:62021",
		[]*hasher.Hasher{hasher.NewHasher([]string{"localhost:62020"})},
		testhelpers.Logger(),
	)
	go proxy.Start()
	WaitForServerStart("62020", "/")

	receivedChan := Client(t, "62021", "/?app=myApp")

	messages := ""
	for message := range receivedChan {
		messages = messages + string(message)
	}
	assert.Contains(t, messages, "Hello World from the serverHello World from the server")
}

func TestProxyingWithHashingBetweenServers(t *testing.T) {
	go Server("localhost:62024", "Hello World from the server 1", 1)
	go Server("localhost:62025", "Hello World from the server 2", 1)

	proxy := NewProxy(
		"localhost:62026",
		[]*hasher.Hasher{hasher.NewHasher([]string{"localhost:62024", "localhost:62025"})},
		testhelpers.Logger(),
	)
	go proxy.Start()
	WaitForServerStart("62024", "/")

	receivedChan := Client(t, "62026", "/?app=myServer1App")

	select {
	case data := <-receivedChan:
		assert.Equal(t, string(data), "Hello World from the server 1")
	case <-time.After(1 * time.Second):
		t.Error("Did not receive response within one second")
	}

	receivedChan = Client(t, "62026", "/?app=myServer2App")

	select {
	case data := <-receivedChan:
		assert.Equal(t, string(data), "Hello World from the server 2")
	case <-time.After(1 * time.Second):
		t.Error("Did not receive response within one second")
	}
}

func TestProxyingWithMultipleAZs(t *testing.T) {
	go Server("localhost:62027", "Hello World from the server 1 - AZ1", 1)
	go Server("localhost:62028", "Hello World from the server 2 - AZ1", 1)
	go Server("localhost:62029", "Hello World from the server 1 - AZ2", 1)
	go Server("localhost:62030", "Hello World from the server 2 - AZ2", 1)

	proxy := NewProxy(
		"localhost:62031",
		[]*hasher.Hasher{
			hasher.NewHasher([]string{"localhost:62027", "localhost:62028"}),
			hasher.NewHasher([]string{"localhost:62029", "localhost:62030"}),
		},
		testhelpers.Logger(),
	)
	go proxy.Start()
	WaitForServerStart("62027", "/")

	receivedChan := Client(t, "62031", "/?app=myServerApp1")
	messages := ""
	for message := range receivedChan {
		messages = messages + string(message)
	}
	assert.Contains(t, messages, "Hello World from the server 1 - AZ1")
	assert.Contains(t, messages, "Hello World from the server 1 - AZ2")

	receivedChan = Client(t, "62031", "/?app=myServer2App")
	messages = ""
	for message := range receivedChan {
		messages = messages + string(message)
	}
	assert.Contains(t, messages, "Hello World from the server 2 - AZ1")
	assert.Contains(t, messages, "Hello World from the server 2 - AZ2")
}

func TestKeepAliveWithMultipleAZs(t *testing.T) {
	keepAliveChan1 := make(chan []byte)
	keepAliveChan2 := make(chan []byte)
	go KeepAliveServer("localhost:62032", keepAliveChan1)
	go KeepAliveServer("localhost:62033", keepAliveChan2)

	proxy := NewProxy(
		"localhost:62034",
		[]*hasher.Hasher{
			hasher.NewHasher([]string{"localhost:62032"}),
			hasher.NewHasher([]string{"localhost:62033"}),
		},
		testhelpers.Logger(),
	)
	go proxy.Start()
	WaitForServerStart("62032", "/")

	KeepAliveClient(t, "62034", "/?app=myServerApp1")

	select {
	case data := <-keepAliveChan1:
		assert.Equal(t, string(data), "keep alive")
	case <-time.After(1 * time.Second):
		t.Error("Did not receive response within one second")
	}
	select {
	case data := <-keepAliveChan2:
		assert.Equal(t, string(data), "keep alive")
	case <-time.After(1 * time.Second):
		t.Error("Did not receive response within one second")
	}
}

func TestAuthorization(t *testing.T) {
	authorizationChannel := make(chan string)
	go authServer("localhost:62035", authorizationChannel)

	proxy := NewProxy(
		"localhost:62037",
		[]*hasher.Hasher{
			hasher.NewHasher([]string{"localhost:62035"})},
		testhelpers.Logger(),
	)
	go proxy.Start()
	WaitForServerStart("62035", "/?poll=true")

	authClient(t, "62037", "/?app=myServer2App", "AUTH HEADER")

	select {
	case receivedAuth := <-authorizationChannel:
		assert.Equal(t, receivedAuth, "AUTH HEADER")
	case <-time.After(1 * time.Second):
		t.Error("Did not receive response within one second")
	}
}

func WaitForServerStart(port string, path string) {
	serverStarted := func() bool {
		config, err := websocket.NewConfig("ws://localhost:"+port+path, "http://localhost")
		_, err = websocket.DialConfig(config)
		if err != nil {
			return false
		}
		return true
	}
	for !serverStarted() {
		time.Sleep(1 * time.Microsecond)
	}
}

func Client(t *testing.T, port string, path string) chan []byte {
	receivedChan := make(chan []byte)
	config, err := websocket.NewConfig("ws://localhost:"+port+path, "http://localhost")
	assert.NoError(t, err)
	ws, err := websocket.DialConfig(config)
	assert.NoError(t, err)

	go func() {
		for {
			var data []byte
			err := websocket.Message.Receive(ws, &data)
			if err != nil {
				close(receivedChan)
				ws.Close()
				return
			}
			receivedChan <- data
		}

	}()
	return receivedChan
}

func Server(apiEndpoint, response string, numberOfMessages int) {
	websocketHandler := func(ws *websocket.Conn) {
		doneChan := make(chan bool)
		defer ws.Close()
		for ii := 1; ii <= numberOfMessages; ii++ {

			go func(i int) {
				time.Sleep(time.Duration(i) * time.Millisecond)
				websocket.Message.Send(ws, []byte(response))
				if i == numberOfMessages {
					doneChan <- true
				}
			}(ii)
		}
		<-doneChan
	}

	err := http.ListenAndServe(apiEndpoint, websocket.Handler(websocketHandler))
	if err != nil {
		panic(err)
	}
}

func KeepAliveServer(apiEndpoint string, keepAliveChan chan []byte) {
	websocketHandler := func(ws *websocket.Conn) {
		doneChan := make(chan bool)
		var keepAlive []byte
		go func() {
			for {
				err := websocket.Message.Receive(ws, &keepAlive)
				if err != nil {
					doneChan <- true
					return
				}
				keepAliveChan <- keepAlive

			}
		}()
		<-doneChan
	}

	err := http.ListenAndServe(apiEndpoint, websocket.Handler(websocketHandler))
	if err != nil {
		panic(err)
	}
}

func KeepAliveClient(t *testing.T, port string, path string) {
	config, err := websocket.NewConfig("ws://localhost:"+port+path, "http://localhost")
	assert.NoError(t, err)
	ws, err := websocket.DialConfig(config)
	assert.NoError(t, err)

	websocket.Message.Send(ws, []byte("keep alive"))
}

func authServer(apiEndpoint string, authChan chan string) {
	websocketHandler := func(ws *websocket.Conn) {
		defer ws.Close()
		req := ws.Request()
		req.ParseForm()
		if req.Form.Get("poll") != "true" {
			header := req.Header.Get("Authorization")
			authChan <- header
		}
	}

	err := http.ListenAndServe(apiEndpoint, websocket.Handler(websocketHandler))
	if err != nil {
		panic(err)
	}
}

func authClient(t *testing.T, port, path, authToken string) {
	config, err := websocket.NewConfig("ws://localhost:"+port+path, "http://localhost")
	config.Header.Add("Authorization", authToken)
	assert.NoError(t, err)
	ws, err := websocket.DialConfig(config)
	assert.NoError(t, err)

	websocket.Message.Send(ws, []byte("keep alive"))
}
