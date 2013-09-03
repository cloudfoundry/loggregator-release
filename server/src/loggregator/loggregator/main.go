package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/agentlistener"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/collectorregistrar"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/routerregistrar"
	"io/ioutil"
	"loggregator/authorization"
	"loggregator/messagestore"
	"loggregator/sink"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	cfcomponent.Config
	Index                  uint
	ApiHost                string
	UaaVerificationKeyFile string
	SystemDomain           string
	SourcePort             uint32
	WebPort                uint32
	LogFilePath            string
	decoder                authorization.TokenDecoder
	MaxRetainedLogMessages int
}

func (c *Config) validate(logger *gosteno.Logger) (err error) {
	if c.SystemDomain == "" {
		return errors.New("Need system domain to register with NATS")
	}

	if c.MaxRetainedLogMessages == 0 {
		return errors.New("Need max number of log messages to retain per application")
	}

	uaaVerificationKey, err := ioutil.ReadFile(c.UaaVerificationKeyFile)
	if err != nil {
		return errors.New(fmt.Sprintf("Can not read UAA verification key from file %s: %s", c.UaaVerificationKeyFile, err))
	}
	c.decoder, err = authorization.NewUaaTokenDecoder(uaaVerificationKey)
	if err != nil {
		return errors.New(fmt.Sprintf("Can not parse UAA verification key: %s", err))
	}
	err = c.Validate(logger)
	return
}

var version = flag.Bool("version", false, "Version info")
var logFilePath = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
var logLevel = flag.Bool("debug", false, "Debug logging")
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

	runtime.GOMAXPROCS(runtime.NumCPU())

	logger := cfcomponent.NewLogger(*logLevel, *logFilePath, "loggregator")

	config := &Config{SourcePort: 3456, WebPort: 8080, UaaVerificationKeyFile: *uaaVerificationKeyFile}
	err := cfcomponent.ReadConfigInto(config, *configFile)
	if err != nil {
		panic(err)
	}
	err = config.validate(logger)
	if err != nil {
		panic(err)
	}

	listener := agentlistener.NewAgentListener(fmt.Sprintf("0.0.0.0:%d", config.SourcePort), logger)
	incomingData := listener.Start()

	authorizer := authorization.NewLogAccessAuthorizer(config.decoder, config.ApiHost)
	sinkServer := sink.NewSinkServer(
		incomingData,
		messagestore.NewMessageStore(config.MaxRetainedLogMessages),
		logger,
		fmt.Sprintf("0.0.0.0:%d", config.WebPort),
		authorizer,
		30*time.Second,
	)

	cfc, err := cfcomponent.NewComponent(
		config.WebPort,
		"LoggregatorServer",
		config.Index,
		&LoggregatorServerHealthMonitor{},
		config.VarzPort,
		[]string{config.VarzUser, config.VarzPass},
		[]instrumentation.Instrumentable{listener, sinkServer},
	)

	if err != nil {
		panic(err)
	}

	rr := routerregistrar.NewRouterRegistrar(config.MbusClient, logger)
	uri := strings.Replace(cfc.IpAddress, ".", "-", -1)
	uri = uri + "-" + strconv.Itoa(int(config.SourcePort)) + "." + config.SystemDomain
	err = rr.RegisterWithRouter(cfc.IpAddress, config.WebPort, []string{uri})
	if err != nil {
		logger.Fatalf("Did not get response from router when greeting. Using default keep-alive for now. Err: %v.", err)
	}

	cr := collectorregistrar.NewCollectorRegistrar(config.MbusClient, logger)
	err = cr.RegisterWithCollector(cfc)
	if err != nil {
		logger.Warnf("Unable to register with collector. Err: %v.", err)
	}

	go func() {
		err := cfc.StartMonitoringEndpoints()
		if err != nil {
			panic(err)
		}
	}()

	go sinkServer.Start()

	killChan := make(chan os.Signal)
	signal.Notify(killChan, os.Kill)

	for {
		select {
		case <-cfcomponent.RegisterGoRoutineDumpSignalChannel():
			cfcomponent.DumpGoRoutine()
		case <-killChan:
			rr.UnregisterFromRouter(cfc.IpAddress, cfc.WebPort, []string{uri})
			break
		}
	}
}
