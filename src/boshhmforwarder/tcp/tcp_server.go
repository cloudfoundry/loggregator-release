package tcp

import (
	"boshhmforwarder/logging"
	"bufio"
	"fmt"
	"net"
	"time"
)

type messageStatistics struct {
	TotalMessagesReceived int
	DeltaMessagesReceived int
}

func Open(port int, dataCh chan<- string) error {
	// listen on all interfaces
	if isPortOccupied(port) {
		return fmt.Errorf("The configured port %d is in use", port)
	}

	var msgStats = new(messageStatistics)

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}

	receiveChan := make(chan interface{}, 1)

	ticker := time.NewTicker(time.Second * 30)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-receiveChan:
				msgStats.DeltaMessagesReceived++
				msgStats.TotalMessagesReceived++
			case <-ticker.C:
				logging.Log.Info(fmt.Sprintf("Total Messages Received: %d, Recent Messages Received: %d", msgStats.TotalMessagesReceived, msgStats.DeltaMessagesReceived))
				msgStats.DeltaMessagesReceived = 0
			}
		}
	}()

	for {
		// accept connection on port
		conn, err := ln.Accept()
		if err != nil {
			return err
		}

		go read(conn, dataCh, receiveChan)
	}
}

func read(conn net.Conn, dataCh chan<- string, receiveChan chan interface{}) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		message := scanner.Text()
		logging.Log.Debugf("Received message on TCP port: %s", message)
		dataCh <- message
		receiveChan <- true
	}
}

func isPortOccupied(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if ln != nil {
		ln.Close()
	}

	return err != nil
}
