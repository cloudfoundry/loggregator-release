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

var delay time.Duration

func init() {
	var err error
	delay, err = time.ParseDuration(os.Getenv("DELAY"))
	if err != nil {
		fmt.Println("Unable to parse DELAY. Setting delay to 1ms.")
		delay = time.Millisecond
	}
}

func main() {
	uuid, err := uuid.NewV4()
	if err != nil {
		log.Fatal(err)
	}
	guid := uuid.String()

	max, err := strconv.Atoi(os.Getenv("MAX"))
	if err != nil {
		fmt.Println("Unable to parse MAX. Running forever.")
	}

	go func() {
		if max == 0 {
			printForever(guid)
		} else {
			printUntil(guid, max)
		}
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
