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
	"time"

	dopplerConf "doppler/config"
	"doppler/iprange"
	metronConfig "metron/config"
	trafficcontrollerConfig "trafficcontroller/config"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

const (
	sharedSecret     = "test-shared-secret"
	availabilityZone = "test-availability-zone"
	jobName          = "test-job-name"
	jobIndex         = "42"

	portRangeStart       = 55000
	portRangeCoefficient = 100
)

const (
	etcdPortOffset = iota
	etcdPeerPortOffset
	dopplerUDPPortOffset
	dopplerTCPPortOffset
	dopplerTLSPortOffset
	dopplerWSPortOffset
	dopplerGRPCPortOffset
	dopplerPPROFPortOffset
	metronPortOffset
	metronPPROFPortOffset
	tcPortOffset
	tcPPROFPortOffset
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
	By("making sure etcd was built")
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

func BuildTestDopplerConfig(etcdClientURL string, metronPort int) dopplerConf.Config {
	dopplerUDPPort := getPort(dopplerUDPPortOffset)
	dopplerTCPPort := getPort(dopplerTCPPortOffset)
	dopplerTLSPort := getPort(dopplerTLSPortOffset)
	dopplerWSPort := getPort(dopplerWSPortOffset)
	dopplerGRPCPort := getPort(dopplerGRPCPortOffset)
	pprofPort := getPort(dopplerPPROFPortOffset)

	return dopplerConf.Config{
		Index:        "42",
		JobName:      "test-job-name",
		Zone:         "test-availability-zone",
		SharedSecret: "test-shared-secret",

		IncomingUDPPort: uint32(dopplerUDPPort),
		IncomingTCPPort: uint32(dopplerTCPPort),
		OutgoingPort:    uint32(dopplerWSPort),
		GRPC: dopplerConf.GRPC{
			Port:     uint16(dopplerGRPCPort),
			CertFile: ServerCertFilePath(),
			KeyFile:  ServerKeyFilePath(),
			CAFile:   CAFilePath(),
		},
		PPROFPort: uint32(pprofPort),

		EtcdUrls:                  []string{etcdClientURL},
		EtcdMaxConcurrentRequests: 10,
		MetronAddress:             fmt.Sprintf("127.0.0.1:%d", metronPort),

		EnableTLSTransport: true,
		TLSListenerConfig: dopplerConf.TLSListenerConfig{
			Port:     uint32(dopplerTLSPort),
			CertFile: ServerCertFilePath(),
			KeyFile:  ServerKeyFilePath(),
			CAFile:   CAFilePath(),
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
}

func StartTestDoppler(conf dopplerConf.Config) (cleanup func(), wsPort, grpcPort int) {
	By("making sure doppler was built")
	dopplerPath := os.Getenv("DOPPLER_BUILD_PATH")
	Expect(dopplerPath).ToNot(BeEmpty())

	filename, err := writeConfigToFile("doppler-config", conf)
	Expect(err).ToNot(HaveOccurred())

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

func SetupMetron(dopplerURI string, grpcPort, udpPort int) (func(), int, func()) {
	By("making sure metron was build")
	metronPath := os.Getenv("METRON_BUILD_PATH")
	Expect(metronPath).ToNot(BeEmpty())

	By("starting metron")
	metronPort := getPort(metronPortOffset)
	metronConf := metronConfig.Config{
		Index:        jobIndex,
		Job:          jobName,
		Zone:         availabilityZone,
		SharedSecret: sharedSecret,

		IncomingUDPPort: metronPort,
		PPROFPort:       uint32(getPort(metronPPROFPortOffset)),
		Deployment:      "deployment",

		DopplerAddr:    fmt.Sprintf("%s:%d", dopplerURI, grpcPort),
		DopplerAddrUDP: fmt.Sprintf("%s:%d", dopplerURI, udpPort),

		GRPC: metronConfig.GRPC{
			CertFile: ClientCertFilePath(),
			KeyFile:  ClientKeyFilePath(),
			CAFile:   CAFilePath(),
		},

		MetricBatchIntervalMilliseconds:  10,
		RuntimeStatsIntervalMilliseconds: 10,
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

	By("waiting for metron to listen")
	Eventually(func() bool {
		conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", metronConf.PPROFPort))
		if err != nil {
			return false
		}
		conn.Close()
		return true
	}).Should(BeTrue())

	return func() {
			os.Remove(metronCfgFile.Name())
			metronSession.Kill().Wait()
		}, metronPort, func() {
			// TODO When we switch to gRPC we should wait until
			// we can connect to it
			time.Sleep(10 * time.Second)
		}
}

func SetupTrafficcontroller(etcdClientURL string, dopplerWSPort, dopplerGRPCPort, metronPort int) (func(), int) {
	By("making sure trafficcontroller was build")
	tcPath := os.Getenv("TRAFFIC_CONTROLLER_BUILD_PATH")
	Expect(tcPath).ToNot(BeEmpty())

	By("starting trafficcontroller")
	tcPort := getPort(tcPortOffset)
	tcConfig := trafficcontrollerConfig.Config{
		Index:   jobIndex,
		JobName: jobName,

		DopplerPort: uint32(dopplerWSPort),
		GRPC: trafficcontrollerConfig.GRPC{
			Port:     uint16(dopplerGRPCPort),
			CertFile: ClientCertFilePath(),
			KeyFile:  ClientKeyFilePath(),
			CAFile:   CAFilePath(),
		},
		OutgoingDropsondePort: uint32(tcPort),
		PPROFPort:             uint32(getPort(tcPPROFPortOffset)),
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
	Eventually(func() bool {
		conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", tcConfig.PPROFPort))
		if err != nil {
			return false
		}
		conn.Close()
		return true
	}).Should(BeTrue())

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

func writeConfigToFile(name string, conf interface{}) (string, error) {
	confFile, err := ioutil.TempFile("", name)
	if err != nil {
		return "", err
	}

	err = json.NewEncoder(confFile).Encode(conf)
	if err != nil {
		return "", err
	}

	err = confFile.Close()
	if err != nil {
		return "", err
	}

	return confFile.Name(), nil
}
