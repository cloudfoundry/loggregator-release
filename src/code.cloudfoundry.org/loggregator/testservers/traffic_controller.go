package testservers

import (
	"fmt"
	"net"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	tcConf "code.cloudfoundry.org/loggregator/trafficcontroller/app"
)

func BuildTrafficControllerConf(etcdClientURL string, dopplerWSPort, dopplerGRPCPort, metronPort int) tcConf.Config {
	tcPort := getTCPPort()
	return tcConf.Config{
		IP: "127.0.0.1",

		DopplerPort: uint32(dopplerWSPort),
		GRPC: tcConf.GRPC{
			Port:     uint16(dopplerGRPCPort),
			CertFile: Cert("trafficcontroller.crt"),
			KeyFile:  Cert("trafficcontroller.key"),
			CAFile:   Cert("loggregator-ca.crt"),
		},
		OutgoingDropsondePort: uint32(tcPort),
		PPROFPort:             uint32(getTCPPort()),
		MetronConfig: tcConf.MetronConfig{
			UDPAddress: fmt.Sprintf("localhost:%d", metronPort),
		},

		EtcdUrls: []string{etcdClientURL}, EtcdMaxConcurrentRequests: 5,

		SystemDomain:   "vcap.me",
		SkipCertVerify: true,

		ApiHost:         "http://127.0.0.1:65530",
		UaaHost:         "http://127.0.0.1:65531",
		UaaClient:       "bob",
		UaaClientSecret: "yourUncle",
	}
}

func StartTrafficController(conf tcConf.Config) (func(), int) {
	By("making sure trafficcontroller was build")
	tcPath := os.Getenv("TRAFFIC_CONTROLLER_BUILD_PATH")
	Expect(tcPath).ToNot(BeEmpty())

	filename, err := writeConfigToFile("trafficcontroller-config", conf)
	Expect(err).ToNot(HaveOccurred())

	By("starting trafficcontroller")
	tcCommand := exec.Command(tcPath, "--disableAccessControl", "--config", filename)
	tcSession, err := gexec.Start(
		tcCommand,
		gexec.NewPrefixedWriter(color("o", "tc", green, cyan), GinkgoWriter),
		gexec.NewPrefixedWriter(color("e", "tc", red, cyan), GinkgoWriter),
	)
	Expect(err).ToNot(HaveOccurred())

	By("waiting for trafficcontroller to listen")
	Eventually(func() bool {
		conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", conf.PPROFPort))
		if err != nil {
			return false
		}
		conn.Close()
		return true
	}, 10).Should(BeTrue())

	return func() {
		os.Remove(filename)
		tcSession.Kill().Wait()
	}, int(conf.OutgoingDropsondePort)
}
