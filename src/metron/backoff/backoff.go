package backoff

import (
	"doppler/sinks/retrystrategy"
	"fmt"
	"log"
	"time"
)

type Adapter interface {
	Connect() error
}

func Connect(adapter Adapter, backoffStrategy retrystrategy.RetryStrategy, maxRetries int) error {
	timer := time.NewTimer(backoffStrategy(0))
	defer timer.Stop()

	numberOfTries := 0
	for {
		err := adapter.Connect()
		if err == nil {
			log.Print("Connected to etcd")
			return nil
		}

		numberOfTries++
		sleepDuration := backoffStrategy(numberOfTries)
		log.Printf("Failed to connect to etcd. Number of tries: %d. Backing off for %v: %s", numberOfTries, sleepDuration, err)

		timer.Reset(sleepDuration)
		<-timer.C

		if numberOfTries >= maxRetries {
			return fmt.Errorf("Failed to connect to etcd after %d tries.", numberOfTries)
		}
	}
}
