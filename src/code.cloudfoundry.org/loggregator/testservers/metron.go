package testservers

import (
	"fmt"
	"os"
	"os/exec"

	"code.cloudfoundry.org/loggregator/metron/app"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func BuildMetronConfig(dopplerURI string, dopplerGRPCPort int) app.Config {
	return app.Config{
		Index: jobIndex,
		Job:   jobName,
		Zone:  availabilityZone,

		Tags: map[string]string{
			"auto-tag-1": "auto-tag-value-1",
			"auto-tag-2": "auto-tag-value-2",
		},

		Deployment: "deployment",

		DopplerAddr:       fmt.Sprintf("%s:%d", dopplerURI, dopplerGRPCPort),
		DopplerAddrWithAZ: fmt.Sprintf("%s.%s:%d", availabilityZone, dopplerURI, dopplerGRPCPort),

		GRPC: app.GRPC{
			CertFile: Cert("metron.crt"),
			KeyFile:  Cert("metron.key"),
			CAFile:   Cert("loggregator-ca.crt"),
		},

		MetricBatchIntervalMilliseconds:  5000,
		RuntimeStatsIntervalMilliseconds: 10,
	}
}

type MetronPorts struct {
	GRPC   int
	UDP    int
	Health int
	PProf  int
}

func StartMetron(conf app.Config) (cleanup func(), mp MetronPorts) {
	By("making sure metron was build")
	metronPath := os.Getenv("METRON_BUILD_PATH")
	Expect(metronPath).ToNot(BeEmpty())

	filename, err := writeConfigToFile("metron-config", conf)
	Expect(err).ToNot(HaveOccurred())

	By("starting metron")
	metronCommand := exec.Command(metronPath, "--config", filename)
	metronSession, err := gexec.Start(
		metronCommand,
		gexec.NewPrefixedWriter(color("o", "metron", green, magenta), GinkgoWriter),
		gexec.NewPrefixedWriter(color("e", "metron", red, magenta), GinkgoWriter),
	)
	Expect(err).ToNot(HaveOccurred())

	By("waiting for metron to listen")
	mp.GRPC = waitForPortBinding("grpc", metronSession.Err)
	mp.UDP = waitForPortBinding("udp", metronSession.Err)
	mp.Health = waitForPortBinding("health", metronSession.Err)
	mp.PProf = waitForPortBinding("pprof", metronSession.Err)

	cleanup = func() {
		os.Remove(filename)
		metronSession.Kill().Wait()
	}
	return cleanup, mp
}
