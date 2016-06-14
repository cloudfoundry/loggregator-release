package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

const delay = time.Millisecond

func main() {
	// create listener so cf know's it's running
	go func() {
		http.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
			rw.WriteHeader(200)
		})
		err := http.ListenAndServe(":"+os.Getenv("PORT"), nil)
		if err != nil {
			panic(err)
		}
	}()

	// generate guid
	uuid, err := uuid.NewV4()
	if err != nil {
		log.Fatal(err)
	}
	guid := uuid.String()

	// emit logs with guid and count
	for i := 0; ; i++ {
		fmt.Printf("logemitter guid: %s msg: %d\n", guid, i)
		time.Sleep(delay)
	}
}
