package main

import (
	"cfcomponent"
	"cfcomponent/instrumentation"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/cloudfoundry/go_cfmessagebus"
	"github.com/cloudfoundry/gosteno"
	"io/ioutil"
	"loggregator/agentlistener"
	"loggregator/sink"
	"os"
	"os/signal"
	"registrar"
	"runtime"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"
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
	VarzPort               uint32
	SourcePort             uint32
	WebPort                uint32
	LogFilePath            string
	decoder                sink.TokenDecoder
	mbusClient             cfmessagebus.MessageBus
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
	return nil
}

var version = flag.Bool("version", false, "Version info")
var logFilePath = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
var logLevel = flag.Bool("v", false, "Verbose logging")
var configFile = flag.String("config", "config/loggregator.json", "Location of the loggregator config json file")
var uaaVerificationKeyFile = flag.String("tokenFile", "config/uaa_token.pub", "Location of the loggregator's uaa public token file")

const versionNumber = `0.0.TRAVIS_BUILD_NUMBER`
const gitSha = `TRAVIS_COMMIT`

type LoggregatorServerHealthMonitor struct {
}

func (hm LoggregatorServerHealthMonitor) Ok() bool {
	return true
}

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("\n\nversion: %s\ngitSha: %s\n\n", versionNumber, gitSha)
		return
	}

	config := &Config{SourcePort: 3456, WebPort: 8080, UaaVerificationKeyFile: *uaaVerificationKeyFile}
	configBytes, err := ioutil.ReadFile(*configFile)
	if err != nil {
		panic(fmt.Sprintf("Can not read config file [%s]: %s", *configFile, err))
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

	listener := agentlistener.NewAgentListener(fmt.Sprintf("0.0.0.0:%d", config.SourcePort), logger)
	incomingData := listener.Start()

	authorizer := sink.NewLogAccessAuthorizer(config.decoder)
	sinkServer := sink.NewSinkServer(incomingData, logger, fmt.Sprintf("0.0.0.0:%d", config.WebPort), "/tail/", config.ApiHost, authorizer, 30*time.Second)

	cfc, err := cfcomponent.NewComponent(
		config.SystemDomain,
		config.WebPort,
		"LoggregatorServer",
		0,
		&LoggregatorServerHealthMonitor{},
		config.VarzPort,
		[]string{config.VarzUser, config.VarzPass},
		[]instrumentation.Instrumentable{listener, sinkServer},
	)

	if err != nil {
		panic(err)
	}

	r := registrar.NewRegistrar(config.mbusClient, logger)
	r.SubscribeToRouterStart(&cfc)
	r.RegisterWithRouter(&cfc)
	r.KeepRegisteringWithRouter(cfc)

	r.SubscribeToComponentDiscover(cfc)
	r.AnnounceComponent(cfc)

	cfc.StartMonitoringEndpoints()

	go sinkServer.Start()

	killChan := make(chan os.Signal)
	signal.Notify(killChan, os.Kill)

	threadDumpChan := make(chan os.Signal)
	signal.Notify(threadDumpChan, syscall.SIGUSR1)

	for {
		select {
		case <-killChan:
			r.UnregisterFromRouter(cfc)
			os.Exit(0)
		case <-threadDumpChan:
			goRoutineProfiles := pprof.Lookup("goroutine")
			goRoutineProfiles.WriteTo(os.Stdout, 2)
		}
	}
}
