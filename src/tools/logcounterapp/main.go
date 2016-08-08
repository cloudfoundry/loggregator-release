// tool to keep track of messages sent for all deployed apps

// usage: . setup.sh && go run main.go

package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
	"tools/logcounterapp/cflib"
	"tools/logcounterapp/config"
	"tools/logcounterapp/logcounter"

	"github.com/cloudfoundry/noaa/consumer"
)

func init() {
	http.DefaultClient.Timeout = 10 * time.Second
	http.DefaultClient.Transport = &http.Transport{
		TLSHandshakeTimeout: time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
}

func main() {
	cfg, err := config.ParseEnv()
	if err != nil {
		log.Fatal(err)
	}
	uaa := &cflib.UAA{
		URL:          cfg.UaaURL,
		Username:     cfg.Username,
		Password:     cfg.Password,
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
	}
	cc := &cflib.CC{
		URL: cfg.ApiURL,
	}
	logcounter := logcounter.New(uaa, cc, cfg)

	go func() {
		if err := logcounter.Start(); err != nil {
			log.Fatal(err)
		}
	}()

	consumer := consumer.New(cfg.DopplerURL, &tls.Config{InsecureSkipVerify: true}, nil)

	fmt.Println("===== Streaming Firehose (will only succeed if you have admin credentials)")

	// notify on ctrl+c
	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)
	go func() {
		for range terminate {
			logcounter.Stop()
		}
	}()

	for {
		authToken, err := uaa.GetAuthToken()
		if err != nil || authToken == "" {
			fmt.Fprintf(os.Stderr, "error getting token %s\n", err)
			continue
		}
		fmt.Println("got new oauth token")
		msgs, errors := consumer.FirehoseWithoutReconnect(cfg.SubscriptionID, authToken)

		go logcounter.HandleMessages(msgs)
		done := logcounter.HandleErrors(errors, terminate, consumer)
		if done {
			return
		}
	}
}
