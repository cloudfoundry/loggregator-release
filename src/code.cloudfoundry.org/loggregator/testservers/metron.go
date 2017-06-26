package testservers

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"

	"code.cloudfoundry.org/loggregator/metron/app"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func BuildMetronConfig(dopplerURI string, dopplerGRPCPort int) app.Config {
	return app.Config{
		Index:      jobIndex,
		Job:        jobName,
		Zone:       availabilityZone,
		Deployment: "deployment",

		// TODO: this should be an option as it is required for only a
		// specific test
		Tags: map[string]string{
			"auto-tag-1": "auto-tag-value-1",
			"auto-tag-2": "auto-tag-value-2",
		},

		DopplerAddr: fmt.Sprintf("%s:%d", dopplerURI, dopplerGRPCPort),

		GRPC: app.GRPC{
			CertFile: Cert("metron.crt"),
			KeyFile:  Cert("metron.key"),
			CAFile:   Cert("loggregator-ca.crt"),
		},

		MetricBatchIntervalMilliseconds:  5000,
		RuntimeStatsIntervalMilliseconds: 10,
	}
}

func StartMetron(conf app.Config) (func(), app.Config, string, func()) {
	By("making sure metron was built")
	metronPath := os.Getenv("METRON_BUILD_PATH")
	Expect(metronPath).ToNot(BeEmpty())

	filename, err := writeConfigToFile("metron-config", conf)
	Expect(err).ToNot(HaveOccurred())

	By("starting metron")
	infoPath := tmpInfoPath("metron")
	metronCommand := exec.Command(
		metronPath,
		"--config", filename,
		"--info-path", infoPath,
	)
	metronSession, err := gexec.Start(
		metronCommand,
		gexec.NewPrefixedWriter(color("o", "metron", green, magenta), GinkgoWriter),
		gexec.NewPrefixedWriter(color("e", "metron", red, magenta), GinkgoWriter),
	)
	Expect(err).ToNot(HaveOccurred())

	return func() {
			os.Remove(infoPath)
			os.Remove(filename)
			metronSession.Kill().Wait()
		}, conf, infoPath, func() {
			By("waiting for metron to listen")
			healthURL := InfoPollString(infoPath, "metron", "health_url")
			Eventually(func() bool {
				r, err := http.Get(healthURL)
				if err != nil {
					return false
				}
				if r.StatusCode != http.StatusOK {
					return false
				}
				return true
			}, 5).Should(BeTrue())
		}
}

func tmpInfoPath(prefix string) string {
	fname, err := ioutil.TempFile("", prefix)
	if err != nil {
		log.Panic(err)
	}
	return fname.Name()
}
