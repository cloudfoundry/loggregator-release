package testservers

import (
	"fmt"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	envstruct "code.cloudfoundry.org/go-envstruct"
	tcConf "code.cloudfoundry.org/loggregator/trafficcontroller/app"
)

func BuildTrafficControllerConf(routerAddr string, agentPort int, logCacheAddr string) tcConf.Config {
	return tcConf.Config{
		IP:                    "127.0.0.1",
		RouterAddrs:           []string{routerAddr},
		HealthAddr:            "localhost:0",
		LogCacheAddr:          logCacheAddr,
		SystemDomain:          "vcap.me",
		SkipCertVerify:        true,
		ApiHost:               "http://127.0.0.1:65530",
		UaaHost:               "http://127.0.0.1:65531",
		UaaCACert:             Cert("loggregator-ca.crt"),
		UaaClient:             "bob",
		UaaClientSecret:       "yourUncle",
		DisableAccessControl:  true,
		OutgoingDropsondePort: 0,
		OutgoingCertFile: Cert("trafficcontroller.crt"),
		OutgoingKeyFile: Cert("trafficcontroller.key"),

		CCTLSClientConfig: tcConf.CCTLSClientConfig{
			CertFile:   Cert("trafficcontroller.crt"),
			KeyFile:    Cert("trafficcontroller.key"),
			CAFile:     Cert("loggregator-ca.crt"),
			ServerName: "cloud-controller",
		},
		Agent: tcConf.Agent{
			UDPAddress: fmt.Sprintf("localhost:%d", agentPort),
		},
		GRPC: tcConf.GRPC{
			CertFile: Cert("trafficcontroller.crt"),
			KeyFile:  Cert("trafficcontroller.key"),
			CAFile:   Cert("loggregator-ca.crt"),
		},
		LogCacheTLSConfig: tcConf.LogCacheTLSConfig{
			CertFile: Cert("log-cache-trafficcontroller.crt"),
			KeyFile:  Cert("log-cache-trafficcontroller.key"),
			CAFile:   Cert("log-cache.crt"),
		},
	}
}

func BuildTrafficControllerConfWithoutLogCache(routerAddr string, agentPort int) tcConf.Config {
	conf := BuildTrafficControllerConf(routerAddr, agentPort, "")
	conf.LogCacheAddr = ""
	conf.LogCacheTLSConfig = tcConf.LogCacheTLSConfig{}
	return conf
}

type TrafficControllerPorts struct {
	WS     int
	Health int
	PProf  int
}

func StartTrafficController(conf tcConf.Config) (cleanup func(), tp TrafficControllerPorts) {
	By("making sure trafficcontroller was built")
	tcPath := os.Getenv("TRAFFIC_CONTROLLER_BUILD_PATH")
	Expect(tcPath).ToNot(BeEmpty())

	By("starting trafficcontroller")
	tcCommand := exec.Command(tcPath)
	tcCommand.Env = envstruct.ToEnv(&conf)
	tcSession, err := gexec.Start(
		tcCommand,
		gexec.NewPrefixedWriter(color("o", "tc", green, cyan), GinkgoWriter),
		gexec.NewPrefixedWriter(color("e", "tc", red, cyan), GinkgoWriter),
	)
	Expect(err).ToNot(HaveOccurred())

	By("waiting for trafficcontroller to listen")
	tp.WS = waitForPortBinding("ws", tcSession.Err)
	tp.Health = waitForPortBinding("health", tcSession.Err)
	tp.PProf = waitForPortBinding("pprof", tcSession.Err)

	cleanup = func() {
		tcSession.Kill().Wait()
	}
	return cleanup, tp
}
