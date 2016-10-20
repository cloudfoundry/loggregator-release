package debugtools_test

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Debugtools", func() {
	Describe("pprof", func() {
		Context("Doppler", func() {
			It("allows connection on pprofPort", func() {
				path := createDopplerConfig()
				dopplerSession := startComponent(
					dopplerExecutablePath,
					"doppler",
					34,
					fmt.Sprintf("--config=%s", path),
				)

				Eventually(func() bool { return checkEndpoint("6060", "debug/pprof", http.StatusOK) }, 5).Should(Equal(true))
				dopplerSession.Kill().Wait()
			})
		})

		Context("Traffic Controller", func() {
			It("allows connection on pprofPort", func() {
				path := createTCConfig()
				tcSession := startComponent(
					trafficControllerExecutablePath,
					"tc",
					34,
					fmt.Sprintf("--config=%s", path),
					"--disableAccessControl",
				)

				Eventually(func() bool { return checkEndpoint("6060", "debug/pprof", http.StatusOK) }, 5).Should(Equal(true))
				tcSession.Kill().Wait()
			})
		})

		Context("Metron Agent", func() {
			It("allows connection on pprofPort", func() {
				path := createMetronConfig()
				metronSession := startComponent(
					metronExecutablePath,
					"metron",
					34,
					fmt.Sprintf("--config=%s", path),
				)

				// Wait for metron
				Eventually(metronSession.Buffer).Should(gbytes.Say("metron started"))

				Eventually(func() bool { return checkEndpoint("6061", "debug/pprof", http.StatusOK) }, 5).Should(Equal(true))
				metronSession.Kill().Wait()
			})
		})
	})
})

func createTCConfig() string {
	js := fmt.Sprintf(`{
		"JobName": "trafficcontroller",
		"Index": "3",
		"EtcdUrls"    : ["http://127.0.0.1:49623"],
		"EtcdMaxConcurrentRequests": 5,
		"OutgoingDropsondePort": %d,
		"DopplerPort": %d,
		"IncomingPort": %d,
		"SkipCertVerify": true,
		"Zone": "z1",
		"Host": "0.0.0.0",
		"ApiHost": "http://127.0.0.1:49632",
		"SystemDomain": "vcap.me",
		"MetronPort": %d,
		"UaaHost": "http://127.0.0.1:49628",
		"UaaClient": "bob",
		"UaaClientSecret": "yourUncle"
	}`, testPort(), testPort(), testPort(), testPort())

	return tmpFile(js)
}

func createDopplerConfig() string {
	js := fmt.Sprintf(`{
		"EtcdUrls": ["http://127.0.0.1:49623"],
		"EtcdMaxConcurrentRequests": 10,
		"Index": "0",
		"IncomingUDPPort": %d,
		"IncomingTCPPort": %d,
		"OutgoingPort": %d,
		"GRPCPort": %d,
		"MaxRetainedLogMessages": 10,
		"MessageDrainBufferSize": 100,
		"SharedSecret": "end_to_end_secret",
		"SinkSkipCertVerify": true,
		"BlackListIps": [],
		"JobName": "doppler_z1",
		"Zone": "z1",
		"ContainerMetricTTLSeconds": 120,
		"SinkInactivityTimeoutSeconds": 120,
		"MetricBatchIntervalMilliseconds": 10,
		"MetronAddress": "127.0.0.1:49625",
		"Syslog"  : "",
		"CollectorRegistrarIntervalMilliseconds": 100
	}`, testPort(), testPort(), testPort(), testPort())

	return tmpFile(js)
}

func createMetronConfig() string {
	js := fmt.Sprintf(`{
		"Index": "42",
		"Job": "test-component",
		"IncomingUDPPort": %d,
		"SharedSecret": "end_to_end_secret",
		"EtcdUrls"    : ["http://127.0.0.1:49623"],
		"EtcdMaxConcurrentRequests": 1,
		"Zone": "z1",
		"Deployment": "deployment-name",
		"LoggregatorDropsondePort": %d,
		"Protocols": ["tcp"],
		"EnableTLSTransport": false,
		"BufferSize": 5000,
		"MetricBatchIntervalMilliseconds": 10,
		"TCPBatchSizeBytes": 1024,
		"TCPBatchIntervalMilliseconds": 10
	}`, testPort(), testPort())

	return tmpFile(js)
}

func testPort() int {
	add, _ := net.ResolveUDPAddr("udp", ":0")
	l, _ := net.ListenUDP("udp", add)
	defer l.Close()
	port := l.LocalAddr().(*net.UDPAddr).Port
	return port
}

func tmpFile(s string) string {
	f, err := ioutil.TempFile(tmpDir, "")
	Expect(err).ToNot(HaveOccurred())
	defer f.Close()
	fmt.Fprint(f, s)
	return f.Name()
}
