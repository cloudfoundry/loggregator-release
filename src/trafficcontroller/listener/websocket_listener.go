package listener

import (
	"github.com/gorilla/websocket"
	"sync"
)

type websocketListener struct {
	sync.WaitGroup
}

func NewWebsocket() *websocketListener {
	return new(websocketListener)
}

func (l *websocketListener) Start(url string, outputChan OutputChannel, stopChan StopChannel) error {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return err
	}

	serverError := make(chan struct{})
	l.Add(2)
	go func() {
		defer l.Done()
		select {
		case <-stopChan:
			conn.Close()
		case <-serverError:
		}
	}()

	go func() {
		defer l.Done()
		defer close(serverError)

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			outputChan <- msg
		}
	}()

	return nil
}

func (l *websocketListener) Wait() {
	l.WaitGroup.Wait()
}
