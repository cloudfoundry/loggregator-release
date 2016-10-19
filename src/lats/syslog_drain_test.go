package lats_test

import (
	"crypto/sha1"
	"fmt"
	"lats/helpers"
	"net"
	"path"
	"strconv"
	"time"

	"code.cloudfoundry.org/localip"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Syslog Drain", func() {
	It("sends envelopes to syslog drains", func() {
		l, port := buildListener()
		drain, key := buildDrain(port)
		cleanup := helpers.WriteToEtcd(config.EtcdUrls, key, drain)
		defer cleanup()

		var conn net.Conn
		var err error
		f := func() net.Conn {
			// Syslog sink will not connect unless there are messages
			// flowing through doppler
			env := createLogMessage("test-id")
			helpers.EmitToMetron(env)
			conn, err = l.Accept()
			if err != nil {
				println("Error accepting conn: ", err.Error())
			}
			return conn
		}
		Eventually(f).ShouldNot(BeNil())
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

func buildListener() (net.Listener, string) {
	listener, err := net.Listen("tcp", ":0")
	Expect(err).ToNot(HaveOccurred())
	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	return listener, port
}

func buildDrain(port string) (string, string) {
	ip, err := localip.LocalIP()
	Expect(err).ToNot(HaveOccurred())
	drain := fmt.Sprintf("syslog://%s:%s", ip, port)
	drainHash := fmt.Sprintf("%x", sha1.Sum([]byte(drain)))
	key := path.Join("/loggregator", "services", "test-id", drainHash)
	return drain, key
}
