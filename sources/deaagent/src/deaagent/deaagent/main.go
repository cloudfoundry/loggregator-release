package main

import (
	"cfcomponent"
	"deaagent"
	"deaagent/loggregatorclient"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	vcap "govarz"
	"instrumentor"
	"io/ioutil"
	"os"
	"strings"
)

type Config struct {
	VarzPort           uint32
	VarzUser           string
	VarzPass           string
	LoggregatorAddress string
}

func (c *Config) validate() (err error) {
	if c.VarzPass == "" || c.VarzUser == "" || c.VarzPort == 0 {
		return errors.New("Need VARZ username/password/port.")
	}

	if c.LoggregatorAddress == "" {
		return errors.New("Need Loggregator address (host:port).")
	}

	return nil
}

var version = flag.Bool("version", false, "Version info")
var logFilePath = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
var logLevel = flag.Bool("v", false, "Verbose logging")
var configFile = flag.String("config", "config/dea_logging_agent.json", "Location of the DEA loggregator agent config json file")
var instancesJsonFilePath = flag.String("instancesFile", "/var/vcap/data/dea_next/db/instances.json", "The DEA instances JSON file")

const versionNumber = `0.0.TRAVIS_BUILD_NUMBER`
const gitSha = `TRAVIS_COMMIT`

type DeaAgentHealthMonitor struct {
}

func (hm DeaAgentHealthMonitor) Ok() bool {
	return true
}

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("\n\nversion: %s\ngitSha: %s\n\n", versionNumber, gitSha)
		return
	}

	// ** Config Setup
	config := &Config{}
	configBytes, err := ioutil.ReadFile(*configFile)
	if err != nil {
		panic(fmt.Sprintf("Can not read config file [%s]: %s", *configFile, err))
	}
	err = json.Unmarshal(configBytes, config)
	if err != nil {
		panic(fmt.Sprintf("Can not parse config file [%s]: %s", *configFile, err))
	}

	err = config.validate()
	if err != nil {
		panic(err)
	}

	// ** END Config Setup

	// ** Steno Setup
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
	logger := gosteno.NewLogger("deaagent")

	// ** END Steno Seteup

	loggregatorClient := loggregatorclient.NewLoggregatorClient(config.LoggregatorAddress, logger, 4096)

	agent := deaagent.NewAgent(*instancesJsonFilePath, logger)

	instrumentable, ok := loggregatorClient.(instrumentor.Instrumentable)

	if !ok {
		panic(fmt.Sprintf("loggregatorClient cannot be converted to instrumentor.Instrumentable"))
	}

	cfc := &cfcomponent.Component{
		IpAddress:         "0.0.0.0",
		SystemDomain:      "",
		WebPort:           0,
		Type:              "LoggregatorServer",
		Index:             0,
		HealthMonitor:     &DeaAgentHealthMonitor{},
		StatusPort:        config.VarzPort,
		StatusCredentials: []string{config.VarzUser, config.VarzPass},
	}

	cfc.StartHealthz()

	varz := &vcap.Varz{
		UniqueVarz: instrumentor.NewVarzStats([]instrumentor.Instrumentable{instrumentable}),
	}

	component := &vcap.VcapComponent{
		Type:        "Loggregator Server",
		Index:       0,
		Host:        fmt.Sprintf("0.0.0.0:%d", config.VarzPort),
		Credentials: []string{config.VarzUser, config.VarzPass},
		Config:      nil,
		Varz:        varz,
		InfoRoutes:  nil,
	}
	vcap.StartComponent(component)

	agent.Start(loggregatorClient)
}
