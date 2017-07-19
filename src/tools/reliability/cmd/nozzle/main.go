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
	host := flag.String("host", "", "the hostname of the blackbox test")
	port := flag.Int("port", 8080, "the port of the running http server")

	uaaAddr := flag.String("uaa-addr", "", "the address of UAA")
	clientID := flag.String("client-id", "", "the UAA client ID")
	clientSecret := flag.String("client-secret", "", "the UAA client secret")

	dataDogAPIKey := flag.String("datadog-key", "", "the API key for a DataDog account")
	logEndpoint := flag.String("logging-endpoint", "", "the address of the logging endpoint")

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

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("server started on %s", addr)

	log.Println(http.ListenAndServe(addr, nil))
}
