package testservers

import (
	"fmt"
	"net"
	"os"
	"os/exec"

	dopplerConf "code.cloudfoundry.org/loggregator/doppler/app"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func BuildDopplerConfig(etcdClientURL string, metronUDPPort, metronGRPCPort int) dopplerConf.Config {
	dopplerUDPPort := getUDPPort()
	dopplerWSPort := getTCPPort()
	dopplerGRPCPort := getTCPPort()
	pprofPort := getTCPPort()

	return dopplerConf.Config{
		Index:        "42",
		JobName:      "test-job-name",
		Zone:         "test-availability-zone",
		IP:           "127.0.0.1",
		SharedSecret: "test-shared-secret",

		IncomingUDPPort: uint32(dopplerUDPPort),
		OutgoingPort:    uint32(dopplerWSPort),
		GRPC: dopplerConf.GRPC{
			Port:     uint16(dopplerGRPCPort),
			CertFile: Cert("doppler.crt"),
			KeyFile:  Cert("doppler.key"),
			CAFile:   Cert("loggregator-ca.crt"),
		},
		PPROFPort: uint32(pprofPort),

		EtcdUrls:                  []string{etcdClientURL},
		EtcdMaxConcurrentRequests: 10,

		MetronConfig: dopplerConf.MetronConfig{
			UDPAddress:  fmt.Sprintf("127.0.0.1:%d", metronUDPPort),
			GRPCAddress: fmt.Sprintf("127.0.0.1:%d", metronGRPCPort),
		},

		MetricBatchIntervalMilliseconds: 10,
		ContainerMetricTTLSeconds:       120,
		MaxRetainedLogMessages:          10,
		MessageDrainBufferSize:          100,
		SinkDialTimeoutSeconds:          10,
		SinkIOTimeoutSeconds:            10,
		SinkInactivityTimeoutSeconds:    120,
		SinkSkipCertVerify:              true,
		UnmarshallerCount:               5,
	}
}

func StartDoppler(conf dopplerConf.Config) (cleanup func(), wsPort, grpcPort int) {
	By("making sure doppler was built")
	dopplerPath := os.Getenv("DOPPLER_BUILD_PATH")
	Expect(dopplerPath).ToNot(BeEmpty())

	filename, err := writeConfigToFile("doppler-config", conf)
	Expect(err).ToNot(HaveOccurred())

	By("starting doppler")
	dopplerCommand := exec.Command(dopplerPath, "--config", filename)
	dopplerSession, err := gexec.Start(
		dopplerCommand,
		gexec.NewPrefixedWriter(color("o", "doppler", green, blue), GinkgoWriter),
		gexec.NewPrefixedWriter(color("e", "doppler", red, blue), GinkgoWriter),
	)
	Expect(err).ToNot(HaveOccurred())

	By("waiting for doppler to listen")
	dopplerStartedFn := func() bool {
		conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", conf.PPROFPort))
		if err != nil {
			return false
		}
		conn.Close()
		return true
	}
	Eventually(dopplerStartedFn).Should(BeTrue())

	cleanup = func() {
		os.Remove(filename)
		dopplerSession.Kill().Wait()
	}

	return cleanup, int(conf.OutgoingPort), int(conf.GRPC.Port)
}
