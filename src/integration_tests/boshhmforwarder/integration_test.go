package integration_tests_test

import (
	"fmt"
	"net"

	"boshhmforwarder/config"

	"boshhmforwarder/valuemetricsender"

	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
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
		udpChannel = make(chan []byte, 100)
		udpServerAddr, err := net.ResolveUDPAddr("udp", ":10001")
		Expect(err).ToNot(HaveOccurred())

		udpServer, err = net.ListenUDP("udp", udpServerAddr)
		Expect(err).ToNot(HaveOccurred())

		go readUdpToCH(udpServer, udpChannel)

		conf = config.Configuration("./integration_config.json")
	})

	AfterEach(func() {
		Expect(udpServer.Close()).To(Succeed())
		Eventually(udpChannel).Should(BeClosed())
	})

	It("Health Monitor messages are forwarded", func(done Done) {
		defer close(done)

		healthMonitorMessage := "put system.healthy 1445463675 1 deployment=bosh-lite index=2 job=etcd role=unknown"

		var conn net.Conn
		var err error
		Eventually(func() error {
			conn, err = net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", conf.IncomingPort))
			return err
		}, 5).ShouldNot(HaveOccurred())

		defer conn.Close()
		beforeLength := len(udpChannel)
		for i := 0; i < 11; i++ {
			_, err = fmt.Fprintf(conn, "%s\n", healthMonitorMessage)
			Expect(err).ToNot(HaveOccurred())
		}

		Eventually(func() int {
			return len(udpChannel) - beforeLength
		}).Should(BeNumerically(">=", 11))

		for i := 0; i < beforeLength; i++ {
			<-udpChannel
		}

		var message []byte
		Eventually(udpChannel).Should(Receive(&message))

		var envelope events.Envelope
		err = proto.Unmarshal(message, &envelope)
		Expect(err).ToNot(HaveOccurred())
		Expect(envelope.Origin).To(Equal(proto.String(valuemetricsender.ForwarderOrigin)))
		Expect(envelope.EventType).To(Equal(events.Envelope_ValueMetric.Enum()))
		Expect(envelope.Timestamp).To(Equal(proto.Int64(1445463675000000000)))
		Expect(envelope.Deployment).To(Equal(proto.String("bosh-lite")))
		Expect(envelope.Job).To(Equal(proto.String("etcd")))
		Expect(envelope.Index).To(Equal(proto.String("2")))
		Expect(envelope.ValueMetric.Name).To(Equal(proto.String("system.healthy")))
		Expect(envelope.ValueMetric.Value).To(Equal(proto.Float64(1)))
		Expect(envelope.ValueMetric.Unit).To(Equal(proto.String("b")))

	}, 7)

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
