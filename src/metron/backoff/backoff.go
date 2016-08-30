package backoff

import (
	"doppler/sinks/retrystrategy"
	"fmt"
	"time"

	"github.com/cloudfoundry/gosteno"
)

type Adapter interface {
	Connect() error
}

func Connect(adapter Adapter, backoffStrategy retrystrategy.RetryStrategy, logger *gosteno.Logger, maxRetries int) error {
	timer := time.NewTimer(backoffStrategy(0))
	defer timer.Stop()

	numberOfTries := 0
	for {
		err := adapter.Connect()
		if err == nil {
			logger.Info("Connected to etcd")
			return nil
		}

		numberOfTries++
		sleepDuration := backoffStrategy(numberOfTries)
		logger.Warnd(map[string]interface{}{
			"error": err.Error(),
		}, fmt.Sprintf("Failed to connect to etcd. Number of tries: %d. Backing off for %v.", numberOfTries, sleepDuration))

		timer.Reset(sleepDuration)
		<-timer.C

		if numberOfTries >= maxRetries {
			return fmt.Errorf("Failed to connect to etcd after %d tries.", numberOfTries)
		}
	}
}
