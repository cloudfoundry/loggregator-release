// tool for generating log load

// usage: curl <endpoint>?cycles=100&delay=1000&text=time2
// delay is ms
// defaults: 10 cycles, 1/sec, "LogSpinner Log Message"

package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"
)

func main() {
	http.HandleFunc("/", rootResponse)
	fmt.Println("listening...")
	err := http.ListenAndServe(":"+os.Getenv("PORT"), nil)
	if err != nil {
		panic(err)
	}
}

func rootResponse(res http.ResponseWriter, req *http.Request) {

	cycleCount, err := strconv.Atoi(req.FormValue("cycles"))
	if cycleCount == 0 || err != nil {
		cycleCount = 10
	}

	delayMS, err := strconv.Atoi(req.FormValue("delay"))
	if delayMS == 0 || err != nil {
		delayMS = 1000
	}

	logText := (req.FormValue("text"))
	if logText == "" {
		logText = "LogSpinner Log Message"
	}

	go outputLog(cycleCount, delayMS, logText)

	fmt.Fprintf(res, "cycles %d, delay %d, text %s\n", cycleCount, delayMS, logText)
}

func outputLog(cycleCount int, delayMS int, logText string) {
	for i := 0; i < cycleCount; i++ {
		fmt.Printf("msg %d %s\n", i+1, logText)
		time.Sleep(time.Duration(delayMS) * time.Millisecond)
	}

}
