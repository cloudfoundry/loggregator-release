package main

import (
	"crypto/rand"
	"flag"
	"log"
	"time"
)

func main() {
	size := flag.Int("size", 1, "size in MB of log message")
	frequency := flag.Duration("frequency", 1*time.Minute, "frequency of logging")
	flag.Parse()

	for {
		log.Println(buildMessage(*size))

		time.Sleep(*frequency)
	}
}

func buildMessage(mb int) string {
	bytes := make([]byte, mb*1000000)
	rand.Read(bytes)

	return string(bytes)
}
