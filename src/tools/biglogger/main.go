package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	size := flag.Int("size", 1, "size in MB of log message")
	frequency := flag.Duration("frequency", 1*time.Minute, "frequency of logging")
	port := flag.Int("port", 8080, "port of the http server")
	flag.Parse()

	go writeLogs(*size, *frequency)

	addr := fmt.Sprintf(":%d", *port)
	http.ListenAndServe(addr, nil)

}

func writeLogs(mb int, f time.Duration) {
	msg := buildMessage(mb)

	for {
		log.Println(msg)

		time.Sleep(f)
	}
}

func buildMessage(mb int) string {
	bytes := make([]byte, mb*1024*1024)
	for i := 0; i < len(bytes); i++ {
		bytes[i] = '*'
	}

	return string(bytes)
}
