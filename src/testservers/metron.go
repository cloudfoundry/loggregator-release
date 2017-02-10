package testservers

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"

	"metron/api"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func BuildMetronConfig(dopplerURI string, dopplerGRPCPort, dopplerUDPPort int) api.Config {
	metronUDPPort := getUDPPort()
	metronGRPCPort := getTCPPort()

	return api.Config{
		Index:        jobIndex,
		Job:          jobName,
		Zone:         availabilityZone,
		SharedSecret: sharedSecret,

		IncomingUDPPort: metronUDPPort,
		PPROFPort:       uint32(getTCPPort()),
		Deployment:      "deployment",

		DopplerAddr:    fmt.Sprintf("%s:%d", dopplerURI, dopplerGRPCPort),
		DopplerAddrUDP: fmt.Sprintf("%s:%d", dopplerURI, dopplerUDPPort),

		GRPC: api.GRPC{
			Port:     uint16(metronGRPCPort),
			CertFile: MetronCertPath(),
			KeyFile:  MetronKeyPath(),
			CAFile:   CACertPath(),
		},

		MetricBatchIntervalMilliseconds:  10,
		RuntimeStatsIntervalMilliseconds: 10,
	}
}

func StartMetron(conf api.Config) (func(), api.Config, func()) {
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
	}).Should(BeTrue())

	return func() {
			os.Remove(filename)
			metronSession.Kill().Wait()
		}, conf, func() {
			// TODO When we switch to gRPC we should wait until
			// we can connect to it
			time.Sleep(10 * time.Second)
		}
}
