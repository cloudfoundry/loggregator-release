package integration_tests

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"

	dopplerConfig "doppler/config"
	"doppler/iprange"
	metronConfig "metron/config"
	trafficcontrollerConfig "trafficcontroller/config"

	"code.cloudfoundry.org/localip"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

const (
	sharedSecret     = "test-shared-secret"
	availabilityZone = "test-availability-zone"
	jobName          = "test-job-name"
	jobIndex         = "42"

	portRangeStart       = 55000
	portRangeCoefficient = 100
	etcdPortOffset       = iota
	etcdPeerPortOffset
	dopplerUDPPortOffset
	dopplerTCPPortOffset
	dopplerTLSPortOffset
	dopplerWSPortOffset
	dopplerGRPCPortOffset
	metronPortOffset
	trafficcontrollerPortOffset
)

const (
	red = 31 + iota
	green
	yellow
	blue
	magenta
	cyan
	colorFmt = "\x1b[%dm[%s]\x1b[%dm[%s]\x1b[0m "
)

func getPort(offset int) int {
	return config.GinkgoConfig.ParallelNode*portRangeCoefficient + portRangeStart + offset
}

func SetupEtcd() (func(), string) {
	By("making sure etcd was build")
	etcdPath := os.Getenv("ETCD_BUILD_PATH")
	Expect(etcdPath).ToNot(BeEmpty())

	By("starting etcd")
	etcdPort := getPort(etcdPortOffset)
	etcdPeerPort := getPort(etcdPeerPortOffset)
	etcdClientURL := fmt.Sprintf("http://localhost:%d", etcdPort)
	etcdPeerURL := fmt.Sprintf("http://localhost:%d", etcdPeerPort)
	etcdDataDir, err := ioutil.TempDir("", "etcd-data")
	Expect(err).ToNot(HaveOccurred())

	etcdCommand := exec.Command(
		etcdPath,
		"--data-dir", etcdDataDir,
		"--listen-client-urls", etcdClientURL,
		"--listen-peer-urls", etcdPeerURL,
		"--advertise-client-urls", etcdClientURL,
	)
	etcdSession, err := gexec.Start(
		etcdCommand,
		gexec.NewPrefixedWriter(color("o", "etcd", green, yellow), GinkgoWriter),
		gexec.NewPrefixedWriter(color("e", "etcd", red, yellow), GinkgoWriter),
	)
	Expect(err).ToNot(HaveOccurred())

	By("waiting for etcd to respond via http")
	Eventually(func() error {
		req, reqErr := http.NewRequest("PUT", etcdClientURL+"/v2/keys/test", strings.NewReader("value=test"))
		if reqErr != nil {
			return reqErr
		}
		resp, reqErr := http.DefaultClient.Do(req)
		if reqErr != nil {
			return reqErr
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusInternalServerError {
			return errors.New(fmt.Sprintf("got %d response from etcd", resp.StatusCode))
		}
		return nil
	}, 10).Should(Succeed())

	return func() {
		os.RemoveAll(etcdDataDir)
		etcdSession.Kill().Wait()
	}, etcdClientURL
}

func SetupDoppler(etcdClientURL string, metronPort int) (cleanup func(), wsPort, grpcPort int) {
	By("making sure doppler was build")
	dopplerPath := os.Getenv("DOPPLER_BUILD_PATH")
	Expect(dopplerPath).ToNot(BeEmpty())

	By("starting doppler")
	dopplerUDPPort := getPort(dopplerUDPPortOffset)
	dopplerTCPPort := getPort(dopplerTCPPortOffset)
	dopplerTLSPort := getPort(dopplerTLSPortOffset)
	dopplerWSPort := getPort(dopplerWSPortOffset)
	dopplerGRPCPort := getPort(dopplerGRPCPortOffset)

	dopplerConf := dopplerConfig.Config{
		Index:        jobIndex,
		JobName:      jobName,
		Zone:         availabilityZone,
		SharedSecret: sharedSecret,

		IncomingUDPPort: uint32(dopplerUDPPort),
		IncomingTCPPort: uint32(dopplerTCPPort),
		OutgoingPort:    uint32(dopplerWSPort),
		GRPC: dopplerConfig.GRPC{
			Port:     uint16(dopplerGRPCPort),
			CertFile: "../fixtures/server.crt",
			KeyFile:  "../fixtures/server.key",
			CAFile:   "../fixtures/loggregator-ca.crt",
		},

		EtcdUrls:                  []string{etcdClientURL},
		EtcdMaxConcurrentRequests: 10,
		MetronAddress:             fmt.Sprintf("127.0.0.1:%d", metronPort),

		EnableTLSTransport: true,
		TLSListenerConfig: dopplerConfig.TLSListenerConfig{
			Port: uint32(dopplerTLSPort),
			// TODO: move these files as source code and write them to tmp files
			CertFile: "../fixtures/server.crt",
			KeyFile:  "../fixtures/server.key",
			CAFile:   "../fixtures/loggregator-ca.crt",
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

	dopplerCfgFile, err := ioutil.TempFile("", "doppler-config")
	Expect(err).ToNot(HaveOccurred())

	err = json.NewEncoder(dopplerCfgFile).Encode(dopplerConf)
	Expect(err).ToNot(HaveOccurred())
	err = dopplerCfgFile.Close()
	Expect(err).ToNot(HaveOccurred())

	dopplerCommand := exec.Command(dopplerPath, "--config", dopplerCfgFile.Name())
	dopplerSession, err := gexec.Start(
		dopplerCommand,
		gexec.NewPrefixedWriter(color("o", "doppler", green, blue), GinkgoWriter),
		gexec.NewPrefixedWriter(color("e", "doppler", red, blue), GinkgoWriter),
	)
	Expect(err).ToNot(HaveOccurred())

	// a terrible hack
	Eventually(dopplerSession.Buffer).Should(gbytes.Say("doppler server started"))

	By("waiting for doppler to listen")
	Eventually(func() error {
		c, reqErr := net.Dial("tcp", fmt.Sprintf(":%d", dopplerWSPort))
		if reqErr == nil {
			c.Close()
		}
		return reqErr
	}, 3).Should(Succeed())

	return func() {
		os.Remove(dopplerCfgFile.Name())
		dopplerSession.Kill().Wait()
	}, dopplerWSPort, dopplerGRPCPort
}

func SetupMetron(etcdClientURL, proto string) (func(), int, func()) {
	By("making sure metron was build")
	metronPath := os.Getenv("METRON_BUILD_PATH")
	Expect(metronPath).ToNot(BeEmpty())

	By("starting metron")
	protocols := metronConfig.Protocols{
		metronConfig.Protocol(proto): struct{}{},
	}
	metronPort := getPort(metronPortOffset)
	metronConf := metronConfig.Config{
		Index:        jobIndex,
		Job:          jobName,
		Zone:         availabilityZone,
		SharedSecret: sharedSecret,

		Protocols:       metronConfig.Protocols(protocols),
		IncomingUDPPort: metronPort,
		Deployment:      "deployment",

		EtcdUrls:                  []string{etcdClientURL},
		EtcdMaxConcurrentRequests: 1,

		MetricBatchIntervalMilliseconds:  10,
		RuntimeStatsIntervalMilliseconds: 10,
		TCPBatchSizeBytes:                1024,
		TCPBatchIntervalMilliseconds:     10,
	}

	switch proto {
	case "udp":
	case "tls":
		metronConf.TLSConfig = metronConfig.TLSConfig{
			CertFile: "../fixtures/client.crt",
			KeyFile:  "../fixtures/client.key",
			CAFile:   "../fixtures/loggregator-ca.crt",
		}
		fallthrough
	case "tcp":
		metronConf.TCPBatchIntervalMilliseconds = 100
		metronConf.TCPBatchSizeBytes = 10240
	}

	metronCfgFile, err := ioutil.TempFile("", "metron-config")
	Expect(err).ToNot(HaveOccurred())

	err = json.NewEncoder(metronCfgFile).Encode(metronConf)
	Expect(err).ToNot(HaveOccurred())
	err = metronCfgFile.Close()
	Expect(err).ToNot(HaveOccurred())

	metronCommand := exec.Command(metronPath, "--debug", "--config", metronCfgFile.Name())
	metronSession, err := gexec.Start(
		metronCommand,
		gexec.NewPrefixedWriter(color("o", "metron", green, magenta), GinkgoWriter),
		gexec.NewPrefixedWriter(color("e", "metron", red, magenta), GinkgoWriter),
	)
	Expect(err).ToNot(HaveOccurred())

	Eventually(metronSession.Buffer).Should(gbytes.Say("metron started"))

	By("waiting for metron to listen")
	Eventually(func() error {
		c, reqErr := net.Dial("udp4", fmt.Sprintf(":%d", metronPort))
		if reqErr == nil {
			c.Close()
		}
		return reqErr
	}, 3).Should(Succeed())

	return func() {
			os.Remove(metronCfgFile.Name())
			metronSession.Kill().Wait()
		}, metronPort, func() {
			Eventually(metronSession.Buffer).Should(gbytes.Say(" from last etcd event, updating writer..."))
		}
}

func SetupTrafficcontroller(etcdClientURL string, dopplerWSPort, dopplerGRPCPort, metronPort int) (func(), int) {
	By("making sure trafficcontroller was build")
	tcPath := os.Getenv("TRAFFIC_CONTROLLER_BUILD_PATH")
	Expect(tcPath).ToNot(BeEmpty())

	By("starting trafficcontroller")
	tcPort := getPort(trafficcontrollerPortOffset)
	tcConfig := trafficcontrollerConfig.Config{
		Index:   jobIndex,
		JobName: jobName,

		DopplerPort: uint32(dopplerWSPort),
		GRPC: trafficcontrollerConfig.GRPC{
			Port:     uint16(dopplerGRPCPort),
			CertFile: "../fixtures/client.crt",
			KeyFile:  "../fixtures/client.key",
			CAFile:   "../fixtures/loggregator-ca.crt",
		},
		OutgoingDropsondePort: uint32(tcPort),
		MetronHost:            "localhost",
		MetronPort:            metronPort,

		EtcdUrls:                  []string{etcdClientURL},
		EtcdMaxConcurrentRequests: 5,

		SystemDomain:   "vcap.me",
		SkipCertVerify: true,

		ApiHost:         "http://127.0.0.1:65530",
		UaaHost:         "http://127.0.0.1:65531",
		UaaClient:       "bob",
		UaaClientSecret: "yourUncle",
	}

	tcCfgFile, err := ioutil.TempFile("", "trafficcontroller-config")
	Expect(err).ToNot(HaveOccurred())

	err = json.NewEncoder(tcCfgFile).Encode(tcConfig)
	Expect(err).ToNot(HaveOccurred())
	err = tcCfgFile.Close()
	Expect(err).ToNot(HaveOccurred())

	tcCommand := exec.Command(tcPath, "--debug", "--disableAccessControl", "--config", tcCfgFile.Name())
	tcSession, err := gexec.Start(
		tcCommand,
		gexec.NewPrefixedWriter(color("o", "tc", green, cyan), GinkgoWriter),
		gexec.NewPrefixedWriter(color("e", "tc", red, cyan), GinkgoWriter),
	)
	Expect(err).ToNot(HaveOccurred())

	By("waiting for trafficcontroller to listen")
	ip, err := localip.LocalIP()
	Expect(err).ToNot(HaveOccurred())
	Eventually(func() error {
		url := fmt.Sprintf("http://%s:%d", ip, tcPort)
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
		}
		return err
	}, 3).Should(Succeed())

	return func() {
		os.Remove(tcCfgFile.Name())
		tcSession.Kill().Wait()
	}, tcPort
}

func color(oe, proc string, oeColor, procColor int) string {
	if config.DefaultReporterConfig.NoColor {
		oeColor = 0
		procColor = 0
	}
	return fmt.Sprintf(colorFmt, oeColor, oe, procColor, proc)
}
