package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/cactus/go-statsd-client/statsd"
)

func main() {
	port := "8125"
	if len(os.Args) > 1 {
		port = os.Args[1]
	}

	client, err := statsd.NewClient(fmt.Sprintf("127.0.0.1:%s", port), "testNamespace")
	if err != nil {
		fmt.Printf("Error connecting to statsd server: %v\n", err.Error())
		return
	}
	defer client.Close()

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		inputs := strings.Split(line, " ")
		if len(inputs) < 3 {
			fmt.Printf("Wrong number of inputs, 3 needed at least\n")
			continue
		}
		statsdType := inputs[0]
		name := inputs[1]

		value, _ := strconv.ParseInt(inputs[2], 10, 0)
		var sampleRate float32 = 1.0
		if len(inputs) == 4 {
			rate, _ := strconv.ParseFloat(inputs[3], 32)
			sampleRate = float32(rate)
		}

		switch statsdType {
		case "count":
			client.Inc(name, value, sampleRate)
		case "gauge":
			client.Gauge(name, value, sampleRate)
		case "gaugedelta":
			client.GaugeDelta(name, value, sampleRate)
		case "timing":
			client.Timing(name, value, sampleRate)
		default:
			fmt.Printf("Unsupported operation: %s\n", statsdType)
		}
	}
}
