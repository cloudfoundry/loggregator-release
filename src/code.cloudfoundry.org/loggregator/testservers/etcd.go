package testservers

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"errors"
	"io/ioutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func StartTestEtcd() (func(), string) {
	By("making sure etcd was built")
	etcdPath := os.Getenv("ETCD_BUILD_PATH")
	Expect(etcdPath).ToNot(BeEmpty())

	By("starting etcd")
	etcdPort := getTCPPort()
	etcdPeerPort := getTCPPort()
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
