// tool for generating log load

// usage: curl <endpoint>?cycles=100&delay=1ms&text=time2
// delay is duration format (https://golang.org/pkg/time/#ParseDuration)
// defaults: 10 cycles, 1 second, "LogSpinner Log Message"

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

	delay, err := time.ParseDuration(req.FormValue("delay"))
	if err != nil {
		delay = 1000 * time.Millisecond
	}

	logText := (req.FormValue("text"))
	if logText == "" {
		logText = "LogSpinner Log Message"
	}

	go outputLog(cycleCount, delay, logText)

	fmt.Fprintf(res, "cycles %d, delay %s, text %s\n", cycleCount, delay, logText)
}

func outputLog(cycleCount int, delay time.Duration, logText string) {

	now := time.Now()
	for i := 0; i < cycleCount; i++ {
		fmt.Printf("msg %d %s\n", i+1, logText)
		time.Sleep(delay)
	}
	done := time.Now()
	diff := done.Sub(now)

	rate := float64(cycleCount) / diff.Seconds()
	fmt.Printf("Duration %s TotalSent %d Rate %f \n", diff.String(), cycleCount, rate)

}
