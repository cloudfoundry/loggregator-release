package testservers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"

	"github.com/onsi/ginkgo/config"
)

const (
	sharedSecret     = "test-shared-secret"
	availabilityZone = "test-availability-zone"
	jobName          = "test-job-name"
	jobIndex         = "42"
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

func getTCPPort() int {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		panic(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func getUDPPort() int {
	addr, err := net.ResolveUDPAddr("udp", "localhost:0")
	if err != nil {
		panic(err)
	}

	c, err := net.ListenUDP("udp", addr)
	if err != nil {
		panic(err)
	}
	defer c.Close()
	return c.LocalAddr().(*net.UDPAddr).Port
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
