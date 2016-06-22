package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/nu7hatch/gouuid"
)

var delay time.Duration

func init() {
	var err error
	delay, err = time.ParseDuration(os.Getenv("DELAY"))
	if err != nil {
		delay = time.Millisecond
	}
}

func main() {
	go func() {
		http.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
			rw.WriteHeader(200)
		})
		err := http.ListenAndServe(":"+os.Getenv("PORT"), nil)
		if err != nil {
			panic(err)
		}
	}()

	uuid, err := uuid.NewV4()
	if err != nil {
		log.Fatal(err)
	}
	guid := uuid.String()

	for i := 0; ; i++ {
		fmt.Printf("logemitter guid: %s msg: %d\n", guid, i)
		time.Sleep(delay)
	}
}
