package integration_test

import (
	"crypto/tls"
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

	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/dropsonde/logs"
	"github.com/cloudfoundry/noaa/consumer"
	"github.com/cloudfoundry/sonde-go/events"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = XDescribe("communicating with doppler over UDP", func() {
	const (
		sharedSecret     = "very secret"
		availabilityZone = "some availability zone"
		jobName          = "integration test"
		jobIndex         = "42"
	)

	It("forwards messages", func() {
		rand.Seed(time.Now().UnixNano())

		By("compiling metron")
		metronPath, err := gexec.Build("metron", "-race")
		Expect(err).ToNot(HaveOccurred())
		defer os.Remove(metronPath)

		By("compiling doppler")
		dopplerPath, err := gexec.Build("doppler", "-race")
		Expect(err).ToNot(HaveOccurred())
		defer os.Remove(dopplerPath)

		By("compiling etcd")
		etcdPath, err := gexec.Build("github.com/coreos/etcd", "-race")
		Expect(err).ToNot(HaveOccurred())
		defer os.Remove(etcdPath)

		By("starting etcd")
		etcdPort := rand.Intn(55536) + 10000
		etcdClientURL := fmt.Sprintf("http://localhost:%d", etcdPort)
		etcdDataDir, err := ioutil.TempDir("", "etcd-data")
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(etcdDataDir)

		etcdCommand := exec.Command(
			etcdPath,
			"--data-dir", etcdDataDir,
			"--listen-client-urls", etcdClientURL,
			"--advertise-client-urls", etcdClientURL,
		)
		etcdCommand.Stdout = gexec.NewPrefixedWriter("[o][etcd]", GinkgoWriter)
		etcdCommand.Stderr = gexec.NewPrefixedWriter("[e][etcd]", GinkgoWriter)
		etcdSession, err := gexec.Start(etcdCommand, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		defer etcdSession.Kill()

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

		By("starting doppler")
		dopplerPort := rand.Intn(55536) + 10000
		dopplerOutgoingPort := rand.Intn(55536) + 10000
		Expect(err).ToNot(HaveOccurred())
		dopplerConf := dopplerConfig.Config{
			IncomingUDPPort:              uint32(dopplerPort),
			OutgoingPort:                 uint32(dopplerOutgoingPort),
			EtcdUrls:                     []string{etcdClientURL},
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
		defer os.Remove(dopplerCfgFile.Name())

		err = json.NewEncoder(dopplerCfgFile).Encode(dopplerConf)
		Expect(err).ToNot(HaveOccurred())
		err = dopplerCfgFile.Close()
		Expect(err).ToNot(HaveOccurred())

		dopplerCommand := exec.Command(dopplerPath, "--debug", "--config", dopplerCfgFile.Name())
		dopplerCommand.Stdout = gexec.NewPrefixedWriter("[o][doppler]", GinkgoWriter)
		dopplerCommand.Stderr = gexec.NewPrefixedWriter("[e][doppler]", GinkgoWriter)
		dopplerSession, err := gexec.Start(dopplerCommand, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		defer dopplerSession.Kill()

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

		By("starting metron")
		metronPort := rand.Intn(55536) + 10000
		metronConf := metronConfig.Config{
			Deployment:                       "deployment",
			Zone:                             availabilityZone,
			Job:                              jobName,
			Index:                            jobIndex,
			IncomingUDPPort:                  metronPort,
			EtcdUrls:                         []string{etcdClientURL},
			SharedSecret:                     sharedSecret,
			Protocols:                        metronConfig.Protocols([]metronConfig.Protocol{"udp"}),
			MetricBatchIntervalMilliseconds:  10,
			RuntimeStatsIntervalMilliseconds: 10,
			EtcdMaxConcurrentRequests:        10,
		}

		metronCfgFile, err := ioutil.TempFile("", "metron-config")
		Expect(err).ToNot(HaveOccurred())
		defer os.Remove(metronCfgFile.Name())

		err = json.NewEncoder(metronCfgFile).Encode(metronConf)
		Expect(err).ToNot(HaveOccurred())
		err = metronCfgFile.Close()
		Expect(err).ToNot(HaveOccurred())

		metronCommand := exec.Command(metronPath, "--debug", "--config", metronCfgFile.Name())
		metronCommand.Stdout = gexec.NewPrefixedWriter("[o][metron]", GinkgoWriter)
		metronCommand.Stderr = gexec.NewPrefixedWriter("[e][metron]", GinkgoWriter)
		metronSession, err := gexec.Start(metronCommand, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		defer metronSession.Kill()

		// an equally terrible hack
		Eventually(metronSession.Buffer).Should(gbytes.Say("metron started"))

		By("waiting for metron to listen")
		Eventually(func() error {
			c, reqErr := net.Dial("udp4", fmt.Sprintf(":%d", metronPort))
			if reqErr == nil {
				c.Close()
			}
			return reqErr
		}, 3).Should(Succeed())

		By("sending message into metron")
		go func() {
			err = dropsonde.Initialize(fmt.Sprintf("localhost:%d", metronPort), "test-origin")
			Expect(err).NotTo(HaveOccurred())
			// since UDP is fire and forget we need to send many messages in the hope that at least one makes it through the system
			for i := 0; i < 100; i++ {
				fmt.Println("SENDING_LOGS_WAS_SCHEDULED")
				err = logs.SendAppLog("test-app-id", "An event happened!", "test-app-id", "0")
				Expect(err).NotTo(HaveOccurred())
				time.Sleep(10 * time.Millisecond) // we need to do some basic rate limiting when sending messages
				fmt.Printf("test messages sent: %d\n", i)
			}
		}()

		By("reading message from doppler")
		consumer := consumer.New(fmt.Sprintf("ws://10.35.33.123:%d", dopplerOutgoingPort), &tls.Config{InsecureSkipVerify: true}, nil)
		msgsCh, errCh := consumer.TailingLogs("test-app-id", "not-used-auth-token")
		Consistently(errCh).ShouldNot(Receive())

		var receivedMessage *events.LogMessage
		Eventually(msgsCh, 30*time.Second).Should(Receive(&receivedMessage))
		Expect(string(receivedMessage.GetMessage())).To(Equal("An event happened!"))
	})
})
