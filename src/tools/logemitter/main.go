package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/nu7hatch/gouuid"
)

var (
	max   int
	delay time.Duration
)

func init() {
	rate, err := strconv.ParseFloat(os.Getenv("RATE"), 64)
	if err != nil {
		rate = 1000
	}

	t, err := time.ParseDuration(os.Getenv("TIME"))
	if err != nil {
		t = 5 * time.Minute
	}

	max = int(float64(rate) * t.Seconds())
	delay = time.Duration(1 / rate * float64(time.Second))
}

func main() {
	uuid, err := uuid.NewV4()
	if err != nil {
		log.Fatal(err)
	}
	guid := uuid.String()

	go func() {
		if max == 0 {
			printForever(guid)
		}
		printUntil(guid, max)
	}()

	http.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(200)
	})
	err = http.ListenAndServe(":"+os.Getenv("PORT"), nil)
	if err != nil {
		log.Fatal(err)
	}
}

func printForever(guid string) {
	for i := 0; ; i++ {
		fmt.Printf("logemitter guid: %s msg: %d\n", guid, i)
		time.Sleep(delay)
	}
}

func printUntil(guid string, max int) {
	for i := 0; i < max; i++ {
		fmt.Printf("logemitter guid: %s msg: %d\n", guid, i)
		time.Sleep(delay)
	}
}
