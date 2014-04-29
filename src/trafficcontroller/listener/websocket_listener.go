package listener

import "github.com/gorilla/websocket"

type websocketListener struct {
}

func NewWebsocket() *websocketListener {
	return new(websocketListener)
}

func (l *websocketListener) Start(url string) (<-chan []byte, error) {
	outputChan := make(chan []byte)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		close(outputChan)
		return outputChan, err
	}

	go func() {
		defer close(outputChan)

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			outputChan <- msg
		}
	}()

	return outputChan, err
}

func (l *websocketListener) Stop() {

}
