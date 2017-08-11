package reliability

import (
	"context"
	"crypto/tls"
	"log"

	"github.com/gorilla/websocket"
)

type WorkerClient struct {
	addr       string
	skipVerify bool
	runner     Runner
}

func NewWorkerClient(addr string, skipVerify bool, r Runner) *WorkerClient {
	return &WorkerClient{
		addr:       addr,
		skipVerify: skipVerify,
		runner:     r,
	}
}

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
