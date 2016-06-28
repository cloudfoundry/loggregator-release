package forwarder

import (
	"boshhmforwarder/logging"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type messageForwarder struct {
	dataChan chan string
	sender   ValueMetricSender
}

type messageStatistics struct {
	TotalMessagesSent int
	DeltaMessagesSent int
	TotalErrors       int
	DeltaErrors       int
}

type ValueMetricSender interface {
	SendValueMetric(deployment, job, index, eventName string, secondsSinceEpoch int64, value float64, units string) error
}

var eventNameToUnit = map[string]string{
	"system.healthy":                       "b",
	"system.load.1m":                       "Load",
	"system.cpu.user":                      "Load",
	"system.cpu.sys":                       "Load",
	"system.cpu.wait":                      "Load",
	"system.disk.system.percent":           "Percent",
	"system.disk.system.inode_percent":     "Percent",
	"system.mem.percent":                   "Percent",
	"system.swap.percent":                  "Percent",
	"system.disk.ephemeral.percent":        "Percent",
	"system.disk.ephemeral.inode_percent":  "Percent",
	"system.disk.persistent.percent":       "Percent",
	"system.disk.persistent.inode_percent": "Percent",
	"system.mem.kb":                        "Kb",
	"system.swap.kb":                       "Kb",
}

func StartMessageForwarder(sender ValueMetricSender) chan<- string {
	dataCh := make(chan string)
	forwarder := &messageForwarder{
		dataChan: dataCh,
		sender:   sender,
	}
	go forwarder.process()
	return dataCh
}

func (m *messageForwarder) process() {
	var messageStatistics = new(messageStatistics)

	ticker := time.NewTicker(time.Second * 30)
	defer ticker.Stop()

	var message string
	for {
		select {
		case message = <-m.dataChan:
			logging.Log.Debug("Received message: " + message)
			tokens := strings.Split(message, " ")

			if len(tokens) < 4 {
				logging.Log.Debug("Less than 4 tokens so ignoring " + message)
				continue
			}

			eventName := tokens[1]
			secondsSinceEpoch, err := strconv.ParseInt(tokens[2], 10, 64)
			if err != nil {
				logging.Log.Errorf("Cannot parse \"%s\" : %v ", message, err)
				continue
			}
			value, err := strconv.ParseFloat(tokens[3], 64)
			if err != nil {
				logging.Log.Errorf("Cannot parse \"%s\" %v : %v", message, value, err)
				continue
			}
			keyValuePairs := buildMap(tokens, 4)

			unit, ok := eventNameToUnit[eventName]
			if !ok {
				logging.Log.Errorf("EventName %s has no known conversion to unit type", eventName)
				unit = "Unknown"
			}

			err = m.sender.SendValueMetric(
				keyValuePairs["deployment"],
				keyValuePairs["job"],
				keyValuePairs["index"],
				eventName,
				secondsSinceEpoch,
				value,
				unit)
			if err != nil {
				messageStatistics.TotalErrors++
				messageStatistics.DeltaErrors++
				logging.Log.Error("Failed to send Value Metric", err)
			} else {
				messageStatistics.TotalMessagesSent++
				messageStatistics.DeltaMessagesSent++
				logging.Log.Debug("Sent message to metron agent")
			}
		case <-ticker.C:
			logging.Log.Info(fmt.Sprintf("Total Messages Sent: %d, Recent Messages Sent: %d, Total Errors: %d, Recent Errors: %d", messageStatistics.TotalMessagesSent, messageStatistics.DeltaMessagesSent, messageStatistics.TotalErrors, messageStatistics.DeltaErrors))
			messageStatistics.DeltaErrors = 0
			messageStatistics.DeltaMessagesSent = 0
		}
	}
}

func buildMap(tokens []string, startAt int) map[string]string {
	parsed := make(map[string]string)

	for i := startAt; i < len(tokens); i++ {
		token := tokens[i]
		split := strings.Split(token, "=")
		value := ""
		if len(split) > 1 {
			value = split[1]
		}
		parsed[split[0]] = value
	}
	return parsed
}
