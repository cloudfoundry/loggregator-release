package testservers

import (
	"doppler/iprange"
	"fmt"
	"net"
	"os"
	"os/exec"

	dopplerConf "doppler/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func BuildDopplerConfig(etcdClientURL string, metronPort int) dopplerConf.Config {
	dopplerUDPPort := getUDPPort(dopplerUDPPortOffset)
	dopplerTCPPort := getTCPPort(dopplerTCPPortOffset)
	dopplerTLSPort := getTCPPort(dopplerTLSPortOffset)
	dopplerWSPort := getTCPPort(dopplerWSPortOffset)
	dopplerGRPCPort := getTCPPort(dopplerGRPCPortOffset)
	pprofPort := getTCPPort(dopplerPPROFPortOffset)

	return dopplerConf.Config{
		Index:        "42",
		JobName:      "test-job-name",
		Zone:         "test-availability-zone",
		SharedSecret: "test-shared-secret",

		IncomingUDPPort: uint32(dopplerUDPPort),
		IncomingTCPPort: uint32(dopplerTCPPort),
		OutgoingPort:    uint32(dopplerWSPort),
		GRPC: dopplerConf.GRPC{
			Port:     uint16(dopplerGRPCPort),
			CertFile: ServerCertFilePath(),
			KeyFile:  ServerKeyFilePath(),
			CAFile:   CAFilePath(),
		},
		PPROFPort: uint32(pprofPort),

		EtcdUrls:                  []string{etcdClientURL},
		EtcdMaxConcurrentRequests: 10,
		MetronAddress:             fmt.Sprintf("127.0.0.1:%d", metronPort),

		EnableTLSTransport: true,
		TLSListenerConfig: dopplerConf.TLSListenerConfig{
			Port:     uint32(dopplerTLSPort),
			CertFile: ServerCertFilePath(),
			KeyFile:  ServerKeyFilePath(),
			CAFile:   CAFilePath(),
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
		BlackListIps:                    make([]iprange.IPRange, 0),
		Syslog:                          "",
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
