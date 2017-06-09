package proxy

import (
	"time"

	"github.com/gorilla/websocket"
)

type KeepAlive struct {
	conn              *websocket.Conn
	pongChan          chan struct{}
	keepAliveInterval time.Duration
}

func NewKeepAlive(conn *websocket.Conn, keepAliveInterval time.Duration) *KeepAlive {
	return &KeepAlive{
		conn:              conn,
		pongChan:          make(chan struct{}, 1),
		keepAliveInterval: keepAliveInterval,
	}
}

func (k *KeepAlive) Run() {
	k.conn.SetPongHandler(k.pongHandler)
	defer k.conn.SetPongHandler(nil)

	timeout := time.NewTimer(k.keepAliveInterval)
	for {
		err := k.conn.WriteControl(websocket.PingMessage, nil, time.Time{})
		if err != nil {
			return
		}
		timeout.Reset(k.keepAliveInterval)
		select {
		case <-k.pongChan:
			time.Sleep(k.keepAliveInterval / 2)
			continue
		case <-timeout.C:
			return
		}
	}
}

func (k *KeepAlive) pongHandler(string) error {
	select {
	case k.pongChan <- struct{}{}:
	default:
	}
	return nil
}
