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
