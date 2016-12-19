package testservers

import (
	"fmt"
	"os"
	"os/exec"
	"net"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	metronConf "metron/config"
)

func BuildMetronConfig(dopplerURI string, grpcPort, udpPort int) metronConf.Config {
	metronPort := getPort(metronPortOffset)
	return metronConf.Config{
		Index:        jobIndex,
		Job:          jobName,
		Zone:         availabilityZone,
		SharedSecret: sharedSecret,

		IncomingUDPPort: metronPort,
		PPROFPort:       uint32(getPort(metronPPROFPortOffset)),
		Deployment:      "deployment",

		DopplerAddr:    fmt.Sprintf("%s:%d", dopplerURI, grpcPort),
		DopplerAddrUDP: fmt.Sprintf("%s:%d", dopplerURI, udpPort),

		GRPC: metronConf.GRPC{
			CertFile: ClientCertFilePath(),
			KeyFile:  ClientKeyFilePath(),
			CAFile:   CAFilePath(),
		},

		MetricBatchIntervalMilliseconds:  10,
		RuntimeStatsIntervalMilliseconds: 10,
	}
}

func StartMetron(conf metronConf.Config) (func(), int, func()) {
	By("making sure metron was build")
	metronPath := os.Getenv("METRON_BUILD_PATH")
	Expect(metronPath).ToNot(BeEmpty())

	filename, err := writeConfigToFile("metron-config", conf)
	Expect(err).ToNot(HaveOccurred())

	By("starting metron")
	metronCommand := exec.Command(metronPath, "--debug", "--config", filename)
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
	}, conf.IncomingUDPPort, func() {
		// TODO When we switch to gRPC we should wait until
		// we can connect to it
		time.Sleep(10 * time.Second)
	}
}
