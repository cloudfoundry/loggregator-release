package trafficcontroller_test

import (
	"net/http"
	"sync"
	"time"
)

type FakeAuthServer struct {
	ApiEndpoint string
	sync.RWMutex
}

func (fakeAuthServer *FakeAuthServer) Start() {
	go func() {
		server := &http.Server{
			Addr:              fakeAuthServer.ApiEndpoint,
			ReadHeaderTimeout: 2 * time.Second,
			Handler:           fakeAuthServer,
		}

		err := server.ListenAndServe()

		if err != nil {
			panic(err)
		}
	}()
}

func (fakeAuthServer *FakeAuthServer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	_, _ = writer.Write([]byte("Hello from the fake AUTH server!!!"))
}
