package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"tools/reliability"
)

func main() {
	uaaAddr := flag.String("uaa-addr", "https://uaa.run.pivotal.io", "the address of UAA")
	clientID := flag.String("client-id", "", "the UAA client ID")
	clientSecret := flag.String("client-secret", "", "the UAA client secret")

	host := flag.String("host", "blackboxtest.cfapps.com", "the hostname of the blackbox test")
	port := flag.Int("port", 8080, "the port of the running http server")

	dataDogAPIKey := flag.String("datadog-key", "", "the API key for a DataDog account")

	logEndpoint := flag.String("logging-endpoint", "wss://doppler.run.pivotal.io:443", "the address of the logging endpoint")

	flag.Parse()

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	uaaClient := reliability.NewUAAClient(
		*clientID,
		*clientSecret,
		*uaaAddr,
		httpClient,
	)

	reporter := reliability.NewDataDogReporter(
		*dataDogAPIKey,
		*host,
		httpClient,
	)

	testRunner := reliability.NewLogReliabilityTestRunner(
		*logEndpoint,
		fmt.Sprintf("blackbox-test-%d", time.Now().Unix()),
		uaaClient,
		reporter,
	)

	http.Handle("/tests", reliability.NewCreateTestHandler(testRunner))

	log.Println(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}
