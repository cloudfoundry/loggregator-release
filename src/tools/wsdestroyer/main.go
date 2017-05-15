package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

var (
	addr          = flag.String("addr", "localhost:8080", "http service address")
	concurrent    = flag.Int("concurrent", 1000, "how many goroutines that will concurrently test doppler")
	appID         = flag.String("app-id", "", "app id to simulate app connections")
	simulateReads = flag.Bool("simulate-reads", false, "read messages off the ws connection")
)

const maxReads = 10

func main() {
	flag.Parse()
	if *appID == "" {
		for i := 0; i < *concurrent; i++ {
			go firehose()
		}
	} else {
		for i := 0; i < *concurrent; i++ {
			go appStream()
		}
	}
	<-make(chan struct{})
}

func firehose() {
	for i := 0; ; i++ {
		firehoseTest("wsdestroyer-sub-id")
		firehoseTest(fmt.Sprintf("wsdestroyer-sub-id-%d", i))
	}
}

func appStream() {
	for i := 0; ; i++ {
		appStreamTest(*appID)
	}
}

func firehoseTest(subID string) {
	u := url.URL{Scheme: "ws", Host: *addr, Path: "/firehose/" + subID}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Print("dial:", err)
		return
	}
	if *simulateReads {
		r := rand.Intn(maxReads)
		for i := 0; i < r; i++ {
			c.SetReadDeadline(time.Now().Add(time.Second))
			_, _, err := c.ReadMessage()
			if err != nil {
				break
			}
		}
	}
	time.Sleep(time.Second)
	c.Close()

	return
}

func appStreamTest(appID string) {
	u := url.URL{Scheme: "ws", Host: *addr, Path: "/apps/" + appID + "/stream"}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Print("dial:", err)
		return
	}
	if *simulateReads {
		r := rand.Intn(maxReads)
		for i := 0; i < r; i++ {
			c.SetReadDeadline(time.Now().Add(time.Second))
			_, _, err := c.ReadMessage()
			if err != nil {
				break
			}
		}
	}
	time.Sleep(time.Second)
	c.Close()

	return
}
