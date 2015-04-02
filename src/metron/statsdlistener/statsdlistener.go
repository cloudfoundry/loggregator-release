package statsdlistener

import (
	"bufio"
	"bytes"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/gosteno"
	"github.com/gogo/protobuf/proto"
)

type StatsdListener struct {
	host     string
	stopChan chan struct{}

	*gosteno.Logger
}

func NewStatsdListener(listenerAddress string, logger *gosteno.Logger, name string) StatsdListener {
	return StatsdListener{
		host:     listenerAddress,
		stopChan: make(chan struct{}),

		Logger: logger,
	}
}

func (l *StatsdListener) Run(outputChan chan *events.Envelope) {
	udpAddr, err := net.ResolveUDPAddr("udp", l.host)
	if err != nil {
		l.Fatalf("Failed to resolve address %s. %s", l.host, err.Error())
	}
	connection, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		l.Fatalf("Failed to start UDP listener. %s", err.Error())
	}

	l.Infof("Listening for statsd on host %s", l.host)

	// Use max UDP size because we don't know how big the message is.
	maxUDPsize := 65535
	readBytes := make([]byte, maxUDPsize)

	go func() {
		<-l.stopChan
		connection.Close()
	}()

	for {
		readCount, senderAddr, err := connection.ReadFrom(readBytes)
		if err != nil {
			l.Debugf("Error while reading. %s", err)
			return
		}
		l.Debugf("StatsdListener: Read %d bytes from address %s", readCount, senderAddr)
		trimmedBytes := make([]byte, readCount)
		copy(trimmedBytes, readBytes[:readCount])

		scanner := bufio.NewScanner(bytes.NewBuffer(trimmedBytes))
		for scanner.Scan() {
			line := scanner.Text()
			envelope, err := parseStat(line)
			if err == nil {
				outputChan <- envelope
			} else {
				l.Warnf("Error parsing stat line \"%s\": %s", line, err.Error())
			}
		}
	}

}

func (l *StatsdListener) Stop() {
	close(l.stopChan)
}

func parseStat(data string) (*events.Envelope, error) {
	parts := strings.Split(data, ":")

	totalName := parts[0]
	nameParts := strings.SplitN(totalName, ".", 2)
	origin := nameParts[0]
	name := nameParts[1]

	totalValue := parts[1]
	valueParts := strings.Split(totalValue, "|")
	valueString := valueParts[0]
	value, err := strconv.ParseFloat(valueString, 64)
	if err != nil {
		return nil, err
	}

	env := &events.Envelope{
		Origin:    &origin,
		Timestamp: proto.Int64(time.Now().UnixNano()),
		EventType: events.Envelope_ValueMetric.Enum(),

		ValueMetric: &events.ValueMetric{
			Name:  &name,
			Value: &value,
			Unit:  proto.String("gauge"),
		},
	}

	return env, nil

}
