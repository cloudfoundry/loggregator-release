package testservers

import (
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"syslog_drain_binder/config"
)

func BuildSyslogDrainBinderConfig(etcdURL, ccAddress string) config.Config {
	return config.Config{
		PollingBatchSize:          5,
		InstanceName:              "test-binder",
		UpdateIntervalSeconds:     1,
		EtcdMaxConcurrentRequests: 10,
		EtcdUrls:                  []string{etcdURL},

		MetronAddress: "localhost:12344",

		CloudControllerAddress: ccAddress,
		CloudControllerTLSConfig: config.MutualTLSConfig{
			CertFile: SyslogDrainBinderCertPath(),
			KeyFile:  SyslogDrainBinderKeyPath(),
			CAFile:   CACertPath(),
		},
		SkipCertVerify: true,
	}
}

func StartSyslogDrainBinder(conf config.Config) (func(), config.Config) {
	By("making sure syslog drain binder was built")
	binderPath := os.Getenv("SYSLOG_DRAIN_BINDER_BUILD_PATH")
	Expect(binderPath).ToNot(BeEmpty())

	filename, err := writeConfigToFile("syslog-drain-binder-config", conf)
	Expect(err).ToNot(HaveOccurred())

	By("starting syslog drain binder")
	binderCommand := exec.Command(binderPath, "--config", filename)
	binderSession, err := gexec.Start(
		binderCommand,
		gexec.NewPrefixedWriter(color("o", "syslogdrainbinder", green, magenta), GinkgoWriter),
		gexec.NewPrefixedWriter(color("e", "syslogdrainbinder", red, magenta), GinkgoWriter),
	)
	Expect(err).ToNot(HaveOccurred())

	return func() {
		os.Remove(filename)
		binderSession.Kill().Wait()
	}, conf
}
