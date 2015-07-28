package main

import (
	"flag"
	"log"
	"net"
	"os"
	"time"

	"fmt"
	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/signature"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

var destination = flag.String("destination", "localhost:3457", "Message destination")
var secret = flag.String("secret", "secret", "Signing secret")
var rate = flag.Int("rate", 1000000000, "Messages per second; defaults to billion (aka as many as possible)")
var size = flag.Int("size", 5, "Bytes of log message (not including envelope)")
var duration = flag.Int("duration", 1, "Number of seconds in flood")

func main() {
	flag.Parse()

	la, err := net.ResolveUDPAddr("udp", *destination)
	if err != nil {
		log.Fatalf("Error resolving loggregator address %s, %s", *destination, err)
	}

	//    connection, err := net.DialUDP("udp", nil, la)
	connection, err := net.ListenPacket("udp4", "")
	if err != nil {
		log.Fatalf("Error opening udp stuff")
	}

	msg := make([]byte, *size, *size)
	envelope, err := emitter.Wrap(&events.LogMessage{
		Message:     msg,
		MessageType: events.LogMessage_OUT.Enum(),
		Timestamp:   proto.Int64(time.Now().UnixNano()),
	}, "origin")
	if err != nil {
		log.Fatal(err.Error())
	}

	buf, err := proto.Marshal(envelope)
	if err != nil {
		log.Fatal(err.Error())
	}

	finalBytes := signature.SignMessage(buf, []byte(*secret))

	//	println("marshal time", t2.Sub(t1).String())

	var i int
	d := time.Duration(*duration)
	time.AfterFunc(d*time.Second, func() {
		fmt.Printf("%d, %d, %d, ", *duration, len(finalBytes), i)
		connection.Close()
		os.Exit(0)
	})

	t := time.NewTicker(time.Second / time.Duration(*rate))
	for {
		<-t.C
		connection.WriteTo(finalBytes, la)
		i++
	}
}
