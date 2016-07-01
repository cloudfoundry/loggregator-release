package main

import (
	"flag"

	"strconv"

	"boshhmforwarder/config"
	"boshhmforwarder/forwarder"
	"boshhmforwarder/handlers"
	"boshhmforwarder/tcp"
	"boshhmforwarder/valuemetricsender"

	"boshhmforwarder/logging"

	"fmt"
	"net"
	"net/http"

	_ "net/http/pprof"

	"github.com/cloudfoundry/dropsonde"
	"github.com/gorilla/mux"
)

func main() {
	configFilePath := flag.String("configPath", "", "path to the configuration file")

	flag.Parse()

	conf := config.Configuration(*configFilePath)

	if len(conf.Syslog) > 0 {
		logging.SetSysLogger(conf.Syslog)
	}
	logging.SetLevel(conf.LogLevel)

	dropsonde.Initialize("localhost:"+strconv.Itoa(conf.MetronPort), valuemetricsender.ForwarderOrigin)

	go func() {
		err := tcp.Open(conf.IncomingPort, forwarder.StartMessageForwarder(valuemetricsender.NewValueMetricSender()))
		if err != nil {
			logging.Log.Panic("Could not open the TCP port", err)
		}
	}()

	logging.Log.Info("Bosh HM forwarder initialized")

	infoHandler := handlers.NewInfoHandler()
	router := mux.NewRouter()
	router.Handle("/info", infoHandler).Methods("GET")

	if conf.DebugPort > 0 {
		go pprofServer(conf.DebugPort)
	}

	logging.Log.Info(fmt.Sprintf("Starting Info Server on port %d", conf.InfoPort))

	err := http.ListenAndServe(net.JoinHostPort("", fmt.Sprintf("%d", conf.InfoPort)), router)
	if err != nil {
		logging.Log.Panic("Failed to start up alerter: ", err)
	}
}

func pprofServer(debugPort int) {
	logging.Log.Infof("Starting Pprof Server on %d", debugPort)
	err := http.ListenAndServe(net.JoinHostPort("localhost", fmt.Sprintf("%d", debugPort)), nil)
	if err != nil {
		logging.Log.Panic("Pprof Server Error", err)
	}
}
