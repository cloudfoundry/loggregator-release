package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"tools/reliability"
)

func main() {
	// these are provided by cloud foundry
	host, err := host()
	if err != nil {
		log.Fatal(err)
	}
	port := os.Getenv("PORT")

	// these should be provided by you
	uaaAddr := os.Getenv("UAA_ADDR")
	clientID := os.Getenv("CLIENT_ID")
	clientSecret := os.Getenv("CLIENT_SECRET")
	dataDogAPIKey := os.Getenv("DATADOG_API_KEY")
	logEndpoint := os.Getenv("LOG_ENDPOINT")

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
		httpClient,
	)

	testRunner := reliability.NewLogReliabilityTestRunner(
		logEndpoint,
		fmt.Sprintf("blackbox-test-%d", time.Now().Unix()),
		uaaClient,
		reporter,
	)

	http.Handle("/tests", reliability.NewCreateTestHandler(testRunner))

	addr := ":" + port
	log.Printf("server started on %s", addr)
	log.Println(http.ListenAndServe(addr, nil))
}

func host() (string, error) {
	appJSON := []byte(os.Getenv("VCAP_APPLICATION"))
	var appData map[string]interface{}
	err := json.Unmarshal(appJSON, &appData)
	if err != nil {
		return "", err
	}
	log.Printf("%#v", appData)
	uris, ok := appData["uris"].([]interface{})
	if !ok {
		return "", errors.New("can not type assert uris to []interface{}")
	}
	if len(uris) == 0 {
		return "", errors.New("no application uri available, required for reporting to datadog")
	}
	return uris[0].(string), nil
}
