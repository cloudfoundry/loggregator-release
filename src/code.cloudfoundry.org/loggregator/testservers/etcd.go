package testservers

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"

	"io/ioutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

func getEtcdPort() int {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		panic(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func StartTestEtcd() (func(), string) {
	By("making sure etcd was built")
	etcdPath := os.Getenv("ETCD_BUILD_PATH")
	Expect(etcdPath).ToNot(BeEmpty())

	var (
		etcdSession   *gexec.Session
		etcdDataDir   string
		etcdClientURL string
	)

	for i := 0; i < 10; i++ {
		By("starting etcd")
		etcdPort := getEtcdPort()
		etcdPeerPort := getEtcdPort()
		etcdClientURL = fmt.Sprintf("http://localhost:%d", etcdPort)
		etcdPeerURL := fmt.Sprintf("http://localhost:%d", etcdPeerPort)
		var err error
		etcdDataDir, err = ioutil.TempDir("", "etcd-data")
		Expect(err).ToNot(HaveOccurred())

		etcdCommand := exec.Command(
			etcdPath,
			"--data-dir", etcdDataDir,
			"--listen-client-urls", etcdClientURL,
			"--listen-peer-urls", etcdPeerURL,
			"--advertise-client-urls", etcdClientURL,
		)
		etcdSession, err = gexec.Start(
			etcdCommand,
			gexec.NewPrefixedWriter(color("o", "etcd", green, yellow), GinkgoWriter),
			gexec.NewPrefixedWriter(color("e", "etcd", red, yellow), GinkgoWriter),
		)
		Expect(err).ToNot(HaveOccurred())

		By("waiting for etcd to listen")
		if etcdPublished(etcdSession.Err) {
			break
		}

		os.RemoveAll(etcdDataDir)
		etcdSession.Kill().Wait()
	}

	return func() {
		os.RemoveAll(etcdDataDir)
		etcdSession.Kill().Wait()
	}, etcdClientURL
}

func etcdPublished(buf *gbytes.Buffer) bool {
	for i := 0; i < 10; i++ {
		data := buf.Contents()
		if bytes.Contains(data, []byte("published")) {
			return true
		}
		time.Sleep(time.Second)
	}
	return false
}
