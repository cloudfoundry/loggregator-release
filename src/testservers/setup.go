package testservers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"github.com/onsi/ginkgo/config"
)

const (
	sharedSecret     = "test-shared-secret"
	availabilityZone = "test-availability-zone"
	jobName          = "test-job-name"
	jobIndex         = "42"

	portRangeStart       = 55000
	portRangeCoefficient = 100
)

const (
	etcdPortOffset = iota
	etcdPeerPortOffset
	dopplerUDPPortOffset
	dopplerTCPPortOffset
	dopplerTLSPortOffset
	dopplerWSPortOffset
	dopplerGRPCPortOffset
	dopplerPPROFPortOffset
	metronPortOffset
	metronPPROFPortOffset
	tcPortOffset
	tcPPROFPortOffset
)

const (
	red = 31 + iota
	green
	yellow
	blue
	magenta
	cyan
	colorFmt = "\x1b[%dm[%s]\x1b[%dm[%s]\x1b[0m "
)

func getPort(offset int) int {
	return config.GinkgoConfig.ParallelNode*portRangeCoefficient + portRangeStart + offset
}

func color(oe, proc string, oeColor, procColor int) string {
	if config.DefaultReporterConfig.NoColor {
		oeColor = 0
		procColor = 0
	}
	return fmt.Sprintf(colorFmt, oeColor, oe, procColor, proc)
}

func writeConfigToFile(name string, conf interface{}) (string, error) {
	confFile, err := ioutil.TempFile("", name)
	if err != nil {
		return "", err
	}

	err = json.NewEncoder(confFile).Encode(conf)
	if err != nil {
		return "", err
	}

	err = confFile.Close()
	if err != nil {
		return "", err
	}

	return confFile.Name(), nil
}
