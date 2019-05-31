package trafficcontroller_test

import (
	"net/http"
	"sync"
)

type FakeAuthServer struct {
	ApiEndpoint string
	sync.RWMutex
}

func (fakeAuthServer *FakeAuthServer) Start() {
	go func() {
		err := http.ListenAndServe(fakeAuthServer.ApiEndpoint, fakeAuthServer)
		if err != nil {
			panic(err)
		}
	}()
}

func (fakeAuthServer *FakeAuthServer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	writer.Write([]byte("Hello from the fake AUTH server!!!"))
}
