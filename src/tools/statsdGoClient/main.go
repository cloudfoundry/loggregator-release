package main

import (
    "github.com/cactus/go-statsd-client/statsd"
    "bufio"
    "os"
    "strings"
    "fmt"
    "strconv"
)

func main() {
    client, err := statsd.NewClient("127.0.0.1:8125", "testNamespace")
    if err != nil {
        fmt.Printf("Error connecting to statsd server: %v\n", err.Error())
        return
    }
    defer client.Close()

    scanner := bufio.NewScanner(os.Stdin)
    for scanner.Scan() {
        line := scanner.Text()
        inputs := strings.Split(line, " ")
        if (len(inputs) < 3) {
            fmt.Printf("Wrong number of inputs, 3 needed at least\n")
            continue
        }
        statsdType := inputs[0]
        name := inputs[1]
        value, _ := strconv.ParseInt(inputs[2], 10, 0)
        switch statsdType {
            case "count":
            var sampleRate float32 = 1.0
            if (len(inputs) == 4) {
                rate, _ := strconv.ParseFloat(inputs[3], 32)
                sampleRate = float32(rate)
            }
            client.Inc(name, value, sampleRate)
            case "gauge":
            client.Gauge(name, value, 1.0)
            case "timing":
            client.Timing(name, value, 1.0)
            default:
            fmt.Printf("Unsupported operation: %s\n", statsdType)
        }
    }
}