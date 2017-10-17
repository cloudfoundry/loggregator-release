package testservers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"regexp"
	"strconv"
	"time"

	"github.com/onsi/ginkgo/config"
	"github.com/onsi/gomega/gbytes"
)

const (
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

func waitForPortBinding(prefix string, buf *gbytes.Buffer) int {
	formattedRegex := fmt.Sprintf(`%s bound to: .*:(\d+)`, prefix)
	re := regexp.MustCompile(formattedRegex)

	for i := 0; i < 10; i++ {
		data := buf.Contents()
		result := re.FindSubmatch(data)
		if len(result) == 2 {
			port, err := strconv.Atoi(string(result[1]))
			if err != nil {
				log.Panicf("unable to parse port number, port: %#v", result[1])
			}
			return port
		}
		time.Sleep(time.Second)
	}
	log.Panicf("timed out waiting for port binding, prefix: %s", prefix)
	return 0
}
