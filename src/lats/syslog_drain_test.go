package lats_test

import (
	"crypto/sha1"
	"fmt"
	"lats/helpers"
	"net"
	"strconv"
	"time"

	"code.cloudfoundry.org/localip"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Syslog Drain", func() {
	It("sends envelopes to syslog drains", func() {
		l, port := buildListener()
		drainKey, drainData := buildDrain(port)
		cleanup := helpers.WriteToEtcd(config.EtcdUrls, drainKey, drainData)
		defer cleanup()

		var conn net.Conn
		var err error
		f := func() net.Conn {
			// Syslog sink will not connect unless there are messages
			// flowing through doppler
			env := createLogMessage("test-id")
			helpers.EmitToMetron(env)

			l.SetDeadline(time.Now().Add(1 * time.Second))
			conn, err = l.Accept()
			if err != nil {
				println("Error accepting conn: ", err.Error())
				return nil
			}
			return conn
		}

		Eventually(f, 10*time.Second, 2*time.Second).ShouldNot(BeNil())
		defer conn.Close()

		env := createLogMessage("test-id")
		helpers.EmitToMetron(env)

		var result []byte
		f2 := func() []byte {
			result = make([]byte, 2048)
			conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			n, err := conn.Read(result)
			if err != nil {
				return nil
			}
			return result[:n]
		}
		Eventually(f2).ShouldNot(BeNil())
		Expect(fmt.Sprintf("%s", result)).To(ContainSubstring("test-log-message"))
	})
})

func buildListener() (*net.TCPListener, string) {
	ip, err := localip.LocalIP()
	Expect(err).ToNot(HaveOccurred())
	tcpAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:0", ip))
	Expect(err).ToNot(HaveOccurred())
	listener, err := net.ListenTCP("tcp", tcpAddr)
	Expect(err).ToNot(HaveOccurred())
	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	return listener, port
}

func buildDrain(port string) (string, string) {
	ip, err := localip.LocalIP()
	Expect(err).ToNot(HaveOccurred())
	url := fmt.Sprintf("syslog://%s:%s", ip, port)
	data := drainData(ip, url)
	key := drainKey("test-id", data)
	return key, data
}

func drainKey(appID, drainData string) string {
	hash := sha1.Sum([]byte(drainData))
	return fmt.Sprintf("/loggregator/v2/services/%s/%x", appID, hash)
}

func drainData(hostname, drainURL string) string {
	return fmt.Sprintf(`{"hostname":"%s","drainURL":"%s"}`, hostname, drainURL)
}
