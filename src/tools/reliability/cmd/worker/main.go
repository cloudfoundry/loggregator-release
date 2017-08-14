package main

import (
	"context"
	"crypto/tls"
	"log"
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
	skipCertVerify := os.Getenv("SKIP_CERT_VERIFY") == "true"

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: skipCertVerify,
			},
		},
	}

	log.Println("Building UAA client")
	uaaClient := reliability.NewUAAClient(
		clientID,
		clientSecret,
		uaaAddr,
		httpClient,
	)

	log.Println("Building DataDog reporter")
	reporter := reliability.NewDataDogReporter(
		dataDogAPIKey,
		host,
		instanceID,
		httpClient,
	)

	log.Println("Building TestRunner")
	testRunner := reliability.NewLogReliabilityTestRunner(
		logEndpoint,
		"blackbox-test-",
		skipCertVerify,
		uaaClient,
		reporter,
	)

	client := reliability.NewWorkerClient(controlServerAddr, skipCertVerify, testRunner)
	log.Println(client.Run(context.Background()))
}
