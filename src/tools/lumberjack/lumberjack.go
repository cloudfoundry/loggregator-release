package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	port := os.Getenv("PORT")

	var i uint64
	s := time.Now()

	log.Print("Starting lumberjack")
	go func() {
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port),
			http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

				d := time.Now().Sub(s)
				r := float64(d) / float64(i)
				fmt.Fprintf(w, "Alive for %s, average message rate %f", d, r)
			})))
	}()

	for {
		n := time.Now().UnixNano()
		fmt.Printf("%d line %d\n", n, i)
		i++
	}
}
