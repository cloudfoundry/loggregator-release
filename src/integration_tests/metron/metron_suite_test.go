package metron_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gexec"
)

func TestMetron(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metron Integration Test Suite")
}

var (
	etcdPath    string
	dopplerPath string
	metronPath  string
)

var _ = BeforeSuite(func() {
	var err error

	metronPath, err = gexec.Build("metron", "-race")
	Expect(err).ToNot(HaveOccurred())

	dopplerPath, err = gexec.Build("doppler", "-race")
	Expect(err).ToNot(HaveOccurred())

	etcdPath, err = gexec.Build("github.com/coreos/etcd", "-race")
	Expect(err).ToNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	os.Remove(etcdPath)
	os.Remove(dopplerPath)
	os.Remove(metronPath)
})
