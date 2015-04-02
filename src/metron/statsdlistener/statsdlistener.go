package statsdlistener

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"regexp"
	"strconv"
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

var statsdRegexp = regexp.MustCompile(`([^.]+)\.([^:]+):([+-]?)(\d+(\.\d+)?)\|(ms|g|c)(\|@(\d+(\.\d+)?))?`)

func parseStat(data string) (*events.Envelope, error) {
	parts := statsdRegexp.FindStringSubmatch(data)

	if len(parts) == 0 {
		return nil, fmt.Errorf("Input line '%s' was not a valid statsd line.", data)
	}

	// complete matched string = parts[0]
	origin := parts[1]
	name := parts[2]
	// increment/decrement = parts[3]
	valueString := parts[4]
	// decimal part of valueString = parts[5]
	statType := parts[6]
	// full sampling substring = parts[7]
	sampleRateString := parts[8]
	// decimal part of sampleRate = parts[9]

	value, _ := strconv.ParseFloat(valueString, 64)

	var sampleRate float64
	if len(sampleRateString) != 0 {
		sampleRate, _ = strconv.ParseFloat(sampleRateString, 64)
	} else {
		sampleRate = 1
	}

	value = value / sampleRate

	var unit string
	switch statType {
	case "ms":
		unit = "ms"
	default:
		unit = "gauge"
	}

	env := &events.Envelope{
		Origin:    &origin,
		Timestamp: proto.Int64(time.Now().UnixNano()),
		EventType: events.Envelope_ValueMetric.Enum(),

		ValueMetric: &events.ValueMetric{
			Name:  &name,
			Value: &value,
			Unit:  &unit,
		},
	}

	return env, nil
}
