package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/cloudfoundry/go_cfmessagebus"
	"github.com/cloudfoundry/gosteno"
	"io/ioutil"
	"loggregator/agentlistener"
	"loggregator/cfsink"
	"loggregator/registrar"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strings"
)

type Config struct {
	ApiHost                string
	UaaVerificationKeyFile string
	SystemDomain           string
	NatsHost               string
	NatsPort               int
	NatsUser               string
	NatsPass               string
	SourceHost             string
	WebHost                string
	LogFilePath            string
	decoder                cfsink.TokenDecoder
	mbusClient             go_cfmessagebus.CFMessageBus
	port                   string
}

func (c *Config) validate(logger *gosteno.Logger) (err error) {
	if c.SystemDomain == "" {
		return errors.New("Need system domain to register with NATS")
	}
	uaaVerificationKey, err := ioutil.ReadFile(c.UaaVerificationKeyFile)
	if err != nil {
		return errors.New(fmt.Sprintf("Can not read UAA verification key from file %s: %s", c.UaaVerificationKeyFile, err))
	}
	c.decoder, err = cfsink.NewUaaTokenDecoder(uaaVerificationKey)
	if err != nil {
		return errors.New(fmt.Sprintf("Can not parse UAA verification key: %s", err))
	}

	c.mbusClient, err = go_cfmessagebus.NewCFMessageBus("NATS")
	if err != nil {
		return errors.New(fmt.Sprintf("Can not create message bus to NATS: %s", err))
	}
	c.mbusClient.Configure(c.NatsHost, c.NatsPort, c.NatsUser, c.NatsPass)
	c.mbusClient.SetLogger(logger)
	err = c.mbusClient.Connect()
	if err != nil {
		return errors.New(fmt.Sprintf("Could not connect to NATS: ", err.Error()))
	}

	_, c.port, err = net.SplitHostPort(c.WebHost)
	if err != nil {
		return errors.New(fmt.Sprintf("Can not determine port: %s", err))
	}
	return nil
}

var version = flag.Bool("version", false, "Version info")
var logFilePath = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
var logLevel = flag.Bool("v", false, "Verbose logging")
var configFile = flag.String("config", "config/loggregator.json", "Location of the loggregator config json file")
var uaaVerificationKeyFile = flag.String("tokenFile", "config/uaa_token.pub", "Location of the loggregator's uaa public token file")

const versionNumber = `0.0.TRAVIS_BUILD_NUMBER`
const gitSha = `TRAVIS_COMMIT`

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("\n\nversion: %s\ngitSha: %s\n\n", versionNumber, gitSha)
		return
	}

	if *configFile == "" {
		panic(fmt.Sprintf("Can not find config file: %s", *configFile))
		return
	}

	config := &Config{SourceHost: "0.0.0.0:3456", WebHost: "0.0.0.0:8080", UaaVerificationKeyFile: *uaaVerificationKeyFile}
	configBytes, err := ioutil.ReadFile(*configFile)
	if err != nil {
		panic(fmt.Sprintf("Can not read config file %s: %s", *configFile, err))
	}
	err = json.Unmarshal(configBytes, config)
	if err != nil {
		panic(fmt.Sprintf("Can not parse config file %s: %s", *configFile, err))
	}

	runtime.GOMAXPROCS(runtime.NumCPU())

	level := gosteno.LOG_INFO

	if *logLevel {
		level = gosteno.LOG_DEBUG
	}

	loggingConfig := &gosteno.Config{
		Sinks:     make([]gosteno.Sink, 1),
		Level:     level,
		Codec:     gosteno.NewJsonCodec(),
		EnableLOC: true}

	if strings.TrimSpace(*logFilePath) == "" {
		loggingConfig.Sinks[0] = gosteno.NewIOSink(os.Stdout)
	} else {
		loggingConfig.Sinks[0] = gosteno.NewFileSink(*logFilePath)
	}
	gosteno.Init(loggingConfig)
	logger := gosteno.NewLogger("loggregator")

	err = config.validate(logger)
	if err != nil {
		panic(err)
	}

	listener := agentlistener.NewAgentListener(config.SourceHost, logger)
	incomingData := listener.Start()

	authorizer := cfsink.NewLogAccessAuthorizer(config.decoder)
	cfSinkServer := cfsink.NewCfSinkServer(incomingData, logger, config.WebHost, "/tail/", config.ApiHost, authorizer)

	r := registrar.NewRegistrar(config.mbusClient, config.SystemDomain, config.port, logger)
	r.SubscribeToRouterStart()
	r.RegisterWithRouter()
	r.KeepRegistering()

	systemChan := make(chan os.Signal)
	signal.Notify(systemChan, os.Kill)

	go cfSinkServer.Start()

	select {
	case <-systemChan:
		r.Unregister()
		os.Exit(0)
	}
}
