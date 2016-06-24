package metron_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	dopplerConfig "doppler/config"
	metronConfig "metron/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

const (
	sharedSecret     = "test-shared-secret"
	availabilityZone = "test-availability-zone"
	jobName          = "test-job-name"
	jobIndex         = "42"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func setupEtcd() (func(), string) {
	By("starting etcd")
	etcdPort := rand.Intn(55536) + 10000
	etcdClientURL := fmt.Sprintf("http://localhost:%d", etcdPort)
	etcdDataDir, err := ioutil.TempDir("", "etcd-data")
	Expect(err).ToNot(HaveOccurred())

	etcdCommand := exec.Command(
		etcdPath,
		"--data-dir", etcdDataDir,
		"--listen-client-urls", etcdClientURL,
		"--advertise-client-urls", etcdClientURL,
	)
	etcdSession, err := gexec.Start(
		etcdCommand,
		gexec.NewPrefixedWriter("[o][etcd]", GinkgoWriter),
		gexec.NewPrefixedWriter("[e][etcd]", GinkgoWriter),
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
		etcdSession.Kill()
	}, etcdClientURL
}

func setupDoppler(etcdClientURL string) (func(), int) {
	By("starting doppler")
	dopplerUDPPort := rand.Intn(55536) + 10000
	dopplerTCPPort := rand.Intn(55536) + 10000
	dopplerTlsPort := rand.Intn(55536) + 10000
	dopplerOutgoingPort := rand.Intn(55536) + 10000

	dopplerConf := dopplerConfig.Config{
		IncomingUDPPort:    uint32(dopplerUDPPort),
		IncomingTCPPort:    uint32(dopplerTCPPort),
		OutgoingPort:       uint32(dopplerOutgoingPort),
		EtcdUrls:           []string{etcdClientURL},
		EnableTLSTransport: true,
		TLSListenerConfig: dopplerConfig.TLSListenerConfig{
			Port:     uint32(dopplerTlsPort),
			CertFile: "../fixtures/server.crt",
			KeyFile:  "../fixtures/server.key",
			CAFile:   "../fixtures/loggregator-ca.crt",
		},
		MaxRetainedLogMessages:       10,
		MessageDrainBufferSize:       100,
		SinkDialTimeoutSeconds:       10,
		SinkIOTimeoutSeconds:         10,
		SinkInactivityTimeoutSeconds: 10,
		UnmarshallerCount:            5,
		Index:                        jobIndex,
		JobName:                      jobName,
		SharedSecret:                 sharedSecret,
		Zone:                         availabilityZone,
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
		gexec.NewPrefixedWriter("[o][doppler]", GinkgoWriter),
		gexec.NewPrefixedWriter("[e][doppler]", GinkgoWriter),
	)
	Expect(err).ToNot(HaveOccurred())

	// a terrible hack
	Eventually(dopplerSession.Buffer).Should(gbytes.Say("doppler server started"))

	By("waiting for doppler to listen")
	Eventually(func() error {
		c, reqErr := net.Dial("tcp", fmt.Sprintf(":%d", dopplerOutgoingPort))
		if reqErr == nil {
			c.Close()
		}
		return reqErr
	}, 3).Should(Succeed())

	return func() {
		os.Remove(dopplerCfgFile.Name())
		dopplerSession.Kill()
	}, dopplerOutgoingPort
}

func setupMetron(etcdClientURL, proto string) (func(), int) {
	By("starting metron")
	protocols := []metronConfig.Protocol{metronConfig.Protocol(proto)}
	metronPort := rand.Intn(55536) + 10000
	metronConf := metronConfig.Config{
		Deployment:                       "deployment",
		Zone:                             availabilityZone,
		Job:                              jobName,
		Index:                            jobIndex,
		IncomingUDPPort:                  metronPort,
		EtcdUrls:                         []string{etcdClientURL},
		SharedSecret:                     sharedSecret,
		MetricBatchIntervalMilliseconds:  10,
		RuntimeStatsIntervalMilliseconds: 10,
		EtcdMaxConcurrentRequests:        10,
		Protocols:                        metronConfig.Protocols(protocols),
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
		gexec.NewPrefixedWriter("[o][metron]", GinkgoWriter),
		gexec.NewPrefixedWriter("[e][metron]", GinkgoWriter),
	)
	Expect(err).ToNot(HaveOccurred())

	Eventually(metronSession.Buffer).Should(gbytes.Say(" from last etcd event, updating writer..."))

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
		metronSession.Kill()
	}, metronPort
}
