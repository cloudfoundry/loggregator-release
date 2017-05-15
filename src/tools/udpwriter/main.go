// udpwriter: a tool that writes messages into a metron via UDP. It is meant
// to stress metron's capability to read and process messages.
//
// There are two writers that spawn when running, a fast writer and a slow
// writer. This allows a variance in the rates of message writes. The slow
// writer will author log messages with the origin of "slow" and the fast
// writer will author log messages with the origin of "fast".
//
package main

import (
	"flag"
	"io"
	"log"
	"net"
	"time"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

var (
	target = flag.String("target", "localhost:3457", "the host:port of the target metron")
	fast   = flag.Duration("fast", time.Second, "the delay of the fast writer")
	slow   = flag.Duration("slow", time.Second, "the delay of the slow writer")
)

func main() {
	flag.Parse()

	conn, err := net.Dial("udp", *target)
	if err != nil {
		log.Fatal(err)
	}

	go writeSlow(conn)
	writeFast(conn)
}

func writeSlow(conn io.Writer) {
	write(*slow, "slow", conn)
}

func writeFast(conn io.Writer) {
	write(*fast, "fast", conn)
}

func write(delay time.Duration, msg string, conn io.Writer) {
	for {
		env := &events.Envelope{
			Origin:    &msg,
			Timestamp: proto.Int64(time.Now().UnixNano()),
			EventType: events.Envelope_LogMessage.Enum(),
			LogMessage: &events.LogMessage{
				Message:     []byte{},
				MessageType: events.LogMessage_OUT.Enum(),
				Timestamp:   proto.Int64(time.Now().UnixNano()),
			},
		}
		envData, err := proto.Marshal(env)
		if err != nil {
			log.Fatal(err)
		}
		_, err = conn.Write(envData)
		if err != nil {
			log.Print(err)
		}
		time.Sleep(delay)
	}
}
