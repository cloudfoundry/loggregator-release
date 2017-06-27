package testservers

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"

	"code.cloudfoundry.org/loggregator/metron/app"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func BuildMetronConfig(dopplerURI string, dopplerGRPCPort int) app.Config {
	metronUDPPort := getUDPPort()
	metronGRPCPort := getTCPPort()

	return app.Config{
		Index: jobIndex,
		Job:   jobName,
		Zone:  availabilityZone,

		Tags: map[string]string{
			"auto-tag-1": "auto-tag-value-1",
			"auto-tag-2": "auto-tag-value-2",
		},

		IncomingUDPPort:    metronUDPPort,
		HealthEndpointPort: uint(7629),
		PPROFPort:          uint32(getTCPPort()),
		Deployment:         "deployment",

		DopplerAddr: fmt.Sprintf("%s:%d", dopplerURI, dopplerGRPCPort),

		GRPC: app.GRPC{
			Port:     uint16(metronGRPCPort),
			CertFile: Cert("metron.crt"),
			KeyFile:  Cert("metron.key"),
			CAFile:   Cert("loggregator-ca.crt"),
		},

		MetricBatchIntervalMilliseconds:  5000,
		RuntimeStatsIntervalMilliseconds: 10,
	}
}

func StartMetron(conf app.Config) (func(), app.Config, func()) {
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
	Eventually(func() bool {
		conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", conf.PPROFPort))
		if err != nil {
			return false
		}
		conn.Close()
		return true
	}, 5).Should(BeTrue())

	return func() {
			os.Remove(filename)
			metronSession.Kill().Wait()
		}, conf, func() {
			// TODO When we switch to gRPC we should wait until
			// we can connect to it
			time.Sleep(10 * time.Second)
		}
}
