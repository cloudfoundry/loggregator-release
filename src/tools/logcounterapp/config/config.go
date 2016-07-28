package config

import (
	"fmt"
	"log"
	"os"

	"github.com/nu7hatch/gouuid"
)

type Config struct {
	ApiURL         string
	DopplerURL     string
	UaaURL         string
	Username       string
	Password       string
	ClientID       string
	ClientSecret   string
	MessagePrefix  string
	SubscriptionID string
	Port           string
}

var envVars = [5]string{"DOPPLER_URL", "API_URL", "UAA_URL", "CLIENT_ID", "PORT"}

func ParseEnv() (*Config, error) {
	for _, env := range envVars {
		value := os.Getenv(env)
		if value == "" {
			return nil, fmt.Errorf("Missing the following environment variable: %s", env)
		}
	}

	subscriptionID := os.Getenv("SUBSCRIPTION_ID")
	if subscriptionID == "" {
		subscriptionID = generateSubscriptionID()
	}

	cfg := &Config{
		ApiURL:         os.Getenv("API_URL"),
		DopplerURL:     os.Getenv("DOPPLER_URL"),
		UaaURL:         os.Getenv("UAA_URL"),
		ClientID:       os.Getenv("CLIENT_ID"),
		ClientSecret:   os.Getenv("CLIENT_SECRET"),
		Username:       os.Getenv("CF_USERNAME"),
		Password:       os.Getenv("CF_PASSWORD"),
		MessagePrefix:  os.Getenv("MESSAGE_PREFIX"),
		SubscriptionID: subscriptionID,
		Port:           os.Getenv("PORT"),
	}

	return cfg, nil
}

func generateSubscriptionID() string {
	guid, err := uuid.NewV4()
	if err != nil {
		log.Fatal(err)
	}
	return guid.String()
}
