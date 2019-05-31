package testservers

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	envstruct "code.cloudfoundry.org/go-envstruct"
	"code.cloudfoundry.org/loggregator/rlp/app"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func BuildRLPConfig(grpcPort, agentGRPCPort int, routerAddrs []string) app.Config {
	return app.Config{
		GRPC: app.GRPC{
			CertFile: Cert("reverselogproxy.crt"),
			KeyFile:  Cert("reverselogproxy.key"),
			CAFile:   Cert("loggregator-ca.crt"),
			Port:     grpcPort,
		},
		MetricEmitterInterval: time.Minute,
		MetricSourceID:        "reverse_log_proxy",
		HealthAddr:            "127.0.0.1:0",
		RouterAddrs:           routerAddrs,
		AgentAddr:             fmt.Sprintf("127.0.0.1:%d", agentGRPCPort),
		MaxEgressStreams:      100,
	}
}

type RLPPorts struct {
	GRPC   int
	PProf  int
	Health int
}

func StartRLP(conf app.Config) (cleanup func(), rp RLPPorts) {
	By("making sure rlp was built")
	rlpPath := os.Getenv("RLP_BUILD_PATH")
	Expect(rlpPath).ToNot(BeEmpty())

	By("starting rlp")
	rlpCommand := exec.Command(rlpPath)
	rlpCommand.Env = envstruct.ToEnv(&conf)

	rlpSession, err := gexec.Start(
		rlpCommand,
		gexec.NewPrefixedWriter(color("o", "rlp", green, blue), GinkgoWriter),
		gexec.NewPrefixedWriter(color("e", "rlp", red, blue), GinkgoWriter),
	)
	Expect(err).ToNot(HaveOccurred())

	By("waiting for rlp to listen")
	rp.GRPC = waitForPortBinding("grpc", rlpSession.Err)
	rp.PProf = waitForPortBinding("pprof", rlpSession.Err)
	rp.Health = waitForPortBinding("health", rlpSession.Err)

	cleanup = func() {
		rlpSession.Kill().Wait()
	}

	return cleanup, rp
}
