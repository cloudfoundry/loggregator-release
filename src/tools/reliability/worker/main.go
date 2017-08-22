package main

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"time"
	"tools/reliability/worker/internal/client"
	"tools/reliability/worker/internal/reporter"

	"github.com/cloudfoundry/noaa/consumer"
)

func main() {
	// these are provided by cloud foundry
	instanceIndex := os.Getenv("CF_INSTANCE_INDEX")

	// these should be provided by you
	uaaAddr := os.Getenv("UAA_ADDR")
	clientID := os.Getenv("CLIENT_ID")
	clientSecret := os.Getenv("CLIENT_SECRET")
	dataDogAPIKey := os.Getenv("DATADOG_API_KEY")
	logEndpoint := os.Getenv("LOG_ENDPOINT")
	controlServerAddr := os.Getenv("CONTROL_SERVER_ADDR")
	host := os.Getenv("HOSTNAME")
	skipCertVerify := os.Getenv("SKIP_CERT_VERIFY") == "true"

	if uaaAddr == "" {
		log.Fatal("UAA_ADDR is required")
	}

	if clientID == "" {
		log.Fatal("CLIENT_ID is required")
	}

	if clientSecret == "" {
		log.Fatal("CLIENT_SECRET is required")
	}

	if dataDogAPIKey == "" {
		log.Fatal("DATADOG_API_KEY is required")
	}

	if logEndpoint == "" {
		log.Fatal("LOG_ENDPOINT is required")
	}

	if controlServerAddr == "" {
		log.Fatal("CONTROL_SERVER_ADDR is required")
	}

	if host == "" {
		log.Fatal("HOSTNAME is required")
	}

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: skipCertVerify,
			},
		},
	}

	log.Println("Building UAA client")
	uaaClient := client.NewUAAClient(
		clientID,
		clientSecret,
		uaaAddr,
		httpClient,
	)

	log.Println("Building DataDog reporter")
	reporter := reporter.NewDataDogReporter(
		dataDogAPIKey,
		host,
		instanceIndex,
		httpClient,
	)

	consumer := consumer.New(logEndpoint, &tls.Config{InsecureSkipVerify: skipCertVerify}, nil)

	log.Println("Building TestRunner")
	testRunner := client.NewLogReliabilityTestRunner(
		logEndpoint,
		"blackbox-test-",
		uaaClient,
		reporter,
		consumer,
	)

	client := client.NewWorkerClient(controlServerAddr, skipCertVerify, testRunner)
	log.Println(client.Run(context.Background()))
}
