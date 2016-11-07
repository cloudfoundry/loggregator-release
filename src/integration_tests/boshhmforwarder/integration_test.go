package integration_tests_test

import (
	"fmt"
	"net"

	"boshhmforwarder/config"

	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Bosh HealthMonitor Forwarder - IntegrationTests", func() {
	var (
		udpServer  *net.UDPConn
		udpChannel chan []byte

		conf *config.Config
	)

	BeforeEach(func() {
		conf = config.Configuration("./integration_config.json")
		udpChannel = make(chan []byte, 100)
		udpServerAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", conf.MetronPort))
		Expect(err).ToNot(HaveOccurred())

		udpServer, err = net.ListenUDP("udp", udpServerAddr)
		Expect(err).ToNot(HaveOccurred())

		go readUdpToCH(udpServer, udpChannel)

	})

	AfterEach(func() {
		Expect(udpServer.Close()).To(Succeed())
		Eventually(udpChannel).Should(BeClosed())
	})

	It("responds to Info requests", func() {
		var resp *http.Response
		Eventually(func() error {
			var err error
			resp, err = http.Get(fmt.Sprintf("http://127.0.0.1:%d/info", conf.InfoPort))
			return err
		}, 5).ShouldNot(HaveOccurred())

		var bodyContents map[string]string

		responseBytes, err := ioutil.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())

		Expect(json.Unmarshal(responseBytes, &bodyContents)).To(Succeed())

		upTime, err := time.ParseDuration(bodyContents["uptime"])
		Expect(err).ToNot(HaveOccurred())

		Expect(upTime.Seconds()).To(BeNumerically("<", 3))
		Expect(upTime.Seconds()).To(BeNumerically(">", 0))
	})
})

func readUdpToCH(connection *net.UDPConn, output chan []byte) {
	defer close(output)

	for {
		buf := make([]byte, 1024)
		n, _, err := connection.ReadFromUDP(buf)
		msg := buf[:n]
		if err != nil {
			return
		}
		output <- msg
	}
}
