package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/cloudfoundry/go_cfmessagebus"
	vcap "github.com/cloudfoundry/gorouter/common"
	"github.com/cloudfoundry/gosteno"
	"instrumentor"
	"io/ioutil"
	"loggregator/agentlistener"
	"loggregator/registrar"
	"loggregator/sink"
	"loggregator/stats"
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
	VarzUser               string
	VarzPass               string
	VarzPort               int
	SourceHost             string
	WebHost                string
	LogFilePath            string
	decoder                sink.TokenDecoder
	mbusClient             cfmessagebus.MessageBus
	port                   string
}

func (c *Config) validate(logger *gosteno.Logger) (err error) {
	if c.VarzPass == "" || c.VarzUser == "" || c.VarzPort == 0 {
		return errors.New("Need VARZ username/password/port.")
	}
	if c.SystemDomain == "" {
		return errors.New("Need system domain to register with NATS")
	}
	uaaVerificationKey, err := ioutil.ReadFile(c.UaaVerificationKeyFile)
	if err != nil {
		return errors.New(fmt.Sprintf("Can not read UAA verification key from file %s: %s", c.UaaVerificationKeyFile, err))
	}
	c.decoder, err = sink.NewUaaTokenDecoder(uaaVerificationKey)
	if err != nil {
		return errors.New(fmt.Sprintf("Can not parse UAA verification key: %s", err))
	}

	c.mbusClient, err = cfmessagebus.NewMessageBus("NATS")
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

	authorizer := sink.NewLogAccessAuthorizer(config.decoder)
	sinkServer := sink.NewSinkServer(incomingData, logger, config.WebHost, "/tail/", config.ApiHost, authorizer)

	r := registrar.NewRegistrar(config.mbusClient, config.SystemDomain, config.port, logger)
	r.SubscribeToRouterStart()
	r.RegisterWithRouter()
	r.KeepRegistering()

	systemChan := make(chan os.Signal)
	signal.Notify(systemChan, os.Kill)

	varz := &vcap.Varz{
		UniqueVarz: stats.NewLoggregatorStats([]instrumentor.Instrumentable{listener}),
	}

	component := &vcap.VcapComponent{
		Type:        "Loggregator Server",
		Index:       0,
		Host:        fmt.Sprintf("0.0.0.0:%d", config.VarzPort),
		Credentials: []string{config.VarzUser, config.VarzPass},
		Config:      nil,
		Logger:      logger,
		Varz:        varz,
		Healthz:     &vcap.Healthz{},
		InfoRoutes:  nil,
	}
	vcap.StartComponent(component)

	go sinkServer.Start()

	select {
	case <-systemChan:
		r.Unregister()
		os.Exit(0)
	}
}
