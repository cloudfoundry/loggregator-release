package testservers

import (
	"fmt"
	"os"
	"os/exec"

	"code.cloudfoundry.org/loggregator/doppler/app"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func BuildDopplerConfig(etcdClientURL string, metronUDPPort, metronGRPCPort int) app.Config {
	return app.Config{
		Index:   "42",
		JobName: "test-job-name",
		Zone:    "test-availability-zone",
		IP:      "127.0.0.1",

		GRPC: app.GRPC{
			CertFile: Cert("doppler.crt"),
			KeyFile:  Cert("doppler.key"),
			CAFile:   Cert("loggregator-ca.crt"),
		},
		HealthAddr: "localhost:0",

		EtcdUrls:                  []string{etcdClientURL},
		EtcdMaxConcurrentRequests: 10,

		MetronConfig: app.MetronConfig{
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
		OutgoingPort:                    8081,
	}
}

type DopplerPorts struct {
	GRPC   int
	PProf  int
	Health int
}

func StartDoppler(conf app.Config) (cleanup func(), dp DopplerPorts) {
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
	dp.GRPC = waitForPortBinding("grpc", dopplerSession.Err)
	dp.PProf = waitForPortBinding("pprof", dopplerSession.Err)
	dp.Health = waitForPortBinding("health", dopplerSession.Err)

	cleanup = func() {
		os.Remove(filename)
		dopplerSession.Kill().Wait()
	}

	return cleanup, dp
}
