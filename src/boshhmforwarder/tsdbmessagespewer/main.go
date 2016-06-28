package main

import (
	"flag"
	"fmt"
	"net"
	"time"
)

func main() {
	host := flag.String("host", "127.0.0.1", "host of the tsdb receiver")
	port := flag.Int("port", 0, "port of the tsdb receiver")
	msgFreq := flag.Int("msgFreq", 0, "number of messages per minute")

	flag.Parse()

	var sleepDurationInSec = (float64(60) / float64(*msgFreq)) * 1000 * 1000 * 1000

	ticker := time.NewTicker(time.Duration(sleepDurationInSec))
	defer ticker.Stop()
	for _ = range ticker.C {
		go func() {
			conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", *host, *port))

			if err != nil {
				panic(err)
			}

			healthMonitorMessage := "put system.healthy 1445463675 1 deployment=metrix-bosh-lite index=2 job=etcd role=unknown"
			defer conn.Close()

			_, err = fmt.Fprintf(conn, "%s\n", healthMonitorMessage)
			if err != nil {
				panic(err)
			}
		}()
	}
}
