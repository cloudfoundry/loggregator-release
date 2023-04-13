package testservers

import (
	"fmt"
	"os"
	"os/exec"

	envstruct "code.cloudfoundry.org/go-envstruct"
	"code.cloudfoundry.org/loggregator-release/src/router/app"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func BuildRouterConfig(agentUDPPort, agentGRPCPort int) app.Config {
	return app.Config{
		GRPC: app.GRPC{
			CertFile: LoggregatorTestCerts.Cert("doppler"),
			KeyFile:  LoggregatorTestCerts.Key("doppler"),
			CAFile:   LoggregatorTestCerts.CA(),
		},

		Agent: app.Agent{
			GRPCAddress: fmt.Sprintf("127.0.0.1:%d", agentGRPCPort),
		},
		IngressBufferSize:               10000,
		EgressBufferSize:                1000,
		MetricBatchIntervalMilliseconds: 10,
	}
}

type RouterPorts struct {
	GRPC  int
	PProf int
}

func StartRouter(conf app.Config) (cleanup func(), rp RouterPorts) {
	By("making sure router was built")
	routerPath := os.Getenv("ROUTER_BUILD_PATH")
	Expect(routerPath).ToNot(BeEmpty())

	By("starting router")
	routerCommand := exec.Command(routerPath)
	routerCommand.Env = envstruct.ToEnv(&conf)

	routerSession, err := gexec.Start(
		routerCommand,
		gexec.NewPrefixedWriter(color("o", "router", green, blue), GinkgoWriter),
		gexec.NewPrefixedWriter(color("e", "router", red, blue), GinkgoWriter),
	)
	Expect(err).ToNot(HaveOccurred())

	By("waiting for router to listen")
	rp.GRPC = waitForPortBinding("grpc", routerSession.Err)
	rp.PProf = waitForPortBinding("pprof", routerSession.Err)

	cleanup = func() {
		routerSession.Kill().Wait()
	}

	return cleanup, rp
}
