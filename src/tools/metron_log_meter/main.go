package main

import (
	"crypto/tls"
	"fmt"
	"github.com/cloudfoundry/noaa"
	"github.com/cloudfoundry/sonde-go/events"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const dopplerAddress = "wss://doppler.pecan.cf-app.com:4443"
const firehoseSubscriptionId = "metronlogmeter"

func main() {

	output, err := exec.Command("cf", "oauth-token").Output()
	outputs := strings.Split(string(output), "bearer")
	if err != nil {
		panic(err)
	}

	authToken := "bearer" + outputs[1]
	msgChan := make(chan *events.Envelope)
	startFirehose(authToken, msgChan)

	logCount, err := strconv.Atoi(os.Args[1])
	if err != nil {
		panic(err)
	}
	delay := os.Args[2]

	callLogSpinner(logCount, delay)

	var totalReceived uint64
	timer := time.NewTimer(40 * time.Second)
loop:
	for {
		select {
		case msg := <-msgChan:
			if msg.GetEventType() == events.Envelope_CounterEvent &&
				msg.GetJob() == "cell_z1" &&
				msg.GetCounterEvent().GetName() == "dropsondeUnmarshaller.logMessageTotal" {

				totalReceived = totalReceived + msg.GetCounterEvent().GetDelta()

				fmt.Printf("Received %d\n", totalReceived)
				fmt.Printf("CounterEvent %v\n", msg.GetCounterEvent())

				if int(totalReceived) == logCount+1 {
					break loop
				}

				fmt.Printf("Resetting...\n")
				timer.Reset(40 * time.Second)
			}
		case <-timer.C:
			break loop
		}
	}

	// Report
	fmt.Printf("Log Count Generated: %d\n", logCount+1)
	fmt.Printf("Log Count Received: %d\n", totalReceived)
	fmt.Printf("Log Loss: %d\n", logCount+1-int(totalReceived))
	fmt.Printf("Log Received (percent): %f\n", (float64(totalReceived)*100)/float64(logCount+1))

}

func callLogSpinner(logCount int, delay string) {
	url := fmt.Sprintf("http://logspinner.pecan.cf-app.com?cycles=%d&delay=%s", logCount, delay)
	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s", respBody)
}

func startFirehose(oauthToken string, msgChan chan *events.Envelope) {
	connection := noaa.NewConsumer(dopplerAddress, &tls.Config{InsecureSkipVerify: true}, nil)
	//	connection.SetDebugPrinter(ConsoleDebugPrinter{})

	fmt.Println("===== Streaming Firehose (will only succeed if you have admin credentials)")

	go func() {
		defer close(msgChan)
		errorChan := make(chan error)
		go connection.Firehose(firehoseSubscriptionId, oauthToken, msgChan, errorChan)

		for err := range errorChan {
			fmt.Fprintf(os.Stderr, "%v\n", err.Error())
		}
	}()
}

type ConsoleDebugPrinter struct{}

func (c ConsoleDebugPrinter) Print(title, dump string) {
	println(title)
	println(dump)
}
