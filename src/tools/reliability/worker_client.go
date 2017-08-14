package reliability

import (
	"context"
	"crypto/tls"
	"log"

	"github.com/gorilla/websocket"
)

// WorkerClient reaches out to the control server to enroll. When tests are
// started, they will be sent via the websocket connection that the
// WorkerClient initiates. The given Runner will be invoked with any tests
// that the control server submits.
type WorkerClient struct {
	addr       string
	skipVerify bool
	runner     Runner
}

// NewWorkerClient builds a new WorkerClient.
func NewWorkerClient(addr string, skipVerify bool, r Runner) *WorkerClient {
	return &WorkerClient{
		addr:       addr,
		skipVerify: skipVerify,
		runner:     r,
	}
}

// Run is used to start the WorkerClient. It starts the websocket connection
// with the control server. The given context controls the lifecycle of the
// Websocket. Each test will be ran (via the Runner) on a new go-routine.
func (w *WorkerClient) Run(ctx context.Context) error {
	dialer := &websocket.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: w.skipVerify,
		},
	}
	conn, _, err := dialer.Dial(w.addr, nil)
	if err != nil {
		return err
	}
	log.Println("connected to control server")

	var cancel func()
	ctx, cancel = context.WithCancel(ctx)
	go func() {
		defer cancel()

		for {
			var test Test
			err := conn.ReadJSON(&test)
			if err != nil {
				break
			}

			log.Println("test received from control server")
			go w.runner.Run(&test)
		}
	}()

	<-ctx.Done()

	return conn.Close()
}
