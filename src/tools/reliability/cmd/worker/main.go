package main

import (
	"context"
	"crypto/tls"
	"net/http"
	"os"
	"time"

	"tools/reliability"
)

func main() {
	// these are provided by cloud foundry
	instanceID := os.Getenv("INSTANCE_GUID")

	// these should be provided by you
	uaaAddr := os.Getenv("UAA_ADDR")
	clientID := os.Getenv("CLIENT_ID")
	clientSecret := os.Getenv("CLIENT_SECRET")
	dataDogAPIKey := os.Getenv("DATADOG_API_KEY")
	logEndpoint := os.Getenv("LOG_ENDPOINT")
	controlServerAddr := os.Getenv("CONTROL_SERVER_ADDR")
	host := os.Getenv("HOSTNAME")

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	uaaClient := reliability.NewUAAClient(
		clientID,
		clientSecret,
		uaaAddr,
		httpClient,
	)

	reporter := reliability.NewDataDogReporter(
		dataDogAPIKey,
		host,
		instanceID,
		httpClient,
	)

	testRunner := reliability.NewLogReliabilityTestRunner(
		logEndpoint,
		"blackbox-test-",
		uaaClient,
		reporter,
	)

	client := reliability.NewWorkerClient(controlServerAddr, testRunner)

	client.Run(context.Background())
}
