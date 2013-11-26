package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

//Test parseConfig

func TestDefaultConfig(t *testing.T) {
	logLevel := false
	configFile := "./test_assets/minimal_loggregator.json"
	logFilePath := "./test_assets/stdout.log"
	config, _ := parseConfig(&logLevel, &configFile, &logFilePath)
	assert.Equal(t, config.IncomingPort, uint32(3456))
	assert.Equal(t, config.OutgoingPort, uint32(8080))
}

func TestParseConfigWorksWithEmptyBlackligtIpProperty(t *testing.T) {
	logLevel := false
	configFile := "./test_assets/minimal_loggregator.json"
	logFilePath := "./test_assets/stdout.log"
	config, _ := parseConfig(&logLevel, &configFile, &logFilePath)
	assert.Nil(t, config.BlackListIps)
}

func TestParseConfigReturnsProperConfigObject(t *testing.T) {
	logLevel := false
	configFile := "./test_assets/loggregator.json"
	logFilePath := "./test_assets/stdout.log"
	config, _ := parseConfig(&logLevel, &configFile, &logFilePath)
	assert.Equal(t, config.IncomingPort, uint32(8765))
	assert.Equal(t, config.OutgoingPort, uint32(4567))
	assert.Equal(t, config.BlackListIps[0].Start, "127.0.0.0")
	assert.Equal(t, config.BlackListIps[0].End, "127.0.0.2")
	assert.Equal(t, config.BlackListIps[1].Start, "127.0.1.12")
	assert.Equal(t, config.BlackListIps[1].End, "127.0.1.15")
}

func TestParseConfigReturnsProperLogger(t *testing.T) {
	configFile := "./test_assets/loggregator.json"
	logFilePath := "./test_assets/stdout.log"

	logLevel := false
	_, logger := parseConfig(&logLevel, &configFile, &logFilePath)
	assert.Equal(t, logger.Level().String(), "info")

	logLevel = true
	_, logger = parseConfig(&logLevel, &configFile, &logFilePath)
	assert.Equal(t, logger.Level().String(), "debug")
}
