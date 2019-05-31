package testservers

import (
	"fmt"
	"os"
	"os/exec"

	envstruct "code.cloudfoundry.org/go-envstruct"
	"code.cloudfoundry.org/loggregator/rlp-gateway/app"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func BuildRLPGatewayConfig(
	gatewayPort int,
	logProviderAddr string,
	logAccessAddr string,
	logAccessExternalAddr string,
	logAdminAddr string,
) app.Config {
	return app.Config{
		LogsProviderAddr:           logProviderAddr,
		LogsProviderCAPath:         Cert("loggregator-ca.crt"),
		LogsProviderClientCertPath: Cert("rlpgateway.crt"),
		LogsProviderClientKeyPath:  Cert("rlpgateway.key"),
		LogsProviderCommonName:     "reverselogproxy",

		HTTP: app.HTTP{
			GatewayAddr: fmt.Sprintf("127.0.0.1:%d", gatewayPort),
		},

		LogAccessAuthorization: app.LogAccessAuthorization{
			Addr:         logAccessAddr,
			CertPath:     Cert("capi.crt"),
			KeyPath:      Cert("capi.key"),
			CAPath:       Cert("loggregator-ca.crt"),
			CommonName:   "capi",
			ExternalAddr: logAccessExternalAddr,
		},

		LogAdminAuthorization: app.LogAdminAuthorization{
			Addr:         logAdminAddr,
			ClientID:     "client",
			ClientSecret: "secret",
			CAPath:       Cert("loggregator-ca.crt"),
		},
	}
}

type RLPGatewayPorts struct {
	HTTP int
}

func StartRLPGateway(conf app.Config) (cleanup func(), rgp RLPGatewayPorts) {
	By("making sure rlp gateway was built")
	rlpGatewayPath := os.Getenv("RLP_GATEWAY_BUILD_PATH")
	Expect(rlpGatewayPath).ToNot(BeEmpty())

	By("starting rlp gateway")
	rlpGatewayCommand := exec.Command(rlpGatewayPath)
	rlpGatewayCommand.Env = envstruct.ToEnv(&conf)

	rlpGatewaySession, err := gexec.Start(
		rlpGatewayCommand,
		gexec.NewPrefixedWriter(color("o", "rlpGateway", green, blue), GinkgoWriter),
		gexec.NewPrefixedWriter(color("e", "rlpGateway", red, blue), GinkgoWriter),
	)
	Expect(err).ToNot(HaveOccurred())

	By("waiting for rlp gateway to listen")
	rgp.HTTP = waitForPortBinding("http", rlpGatewaySession.Err)

	cleanup = func() {
		rlpGatewaySession.Kill().Wait()
	}

	return cleanup, rgp
}
