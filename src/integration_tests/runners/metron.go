package runners

import (
	"encoding/json"
	"io/ioutil"
	"metron/config"
	"net"
	"os/exec"
	"strconv"

	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

type MetronRunner struct {
	Path    string
	TempDir string

	Protocols     config.Protocols
	LegacyPort    int
	MetronPort    int
	DropsondePort int
	EtcdRunner    *etcdstorerunner.ETCDClusterRunner

	CertFile string
	KeyFile  string
	CAFile   string

	Runner  *ginkgomon.Runner
	Process ifrit.Process
}

func (m *MetronRunner) Configure() *ginkgomon.Runner {
	cfgFile, err := ioutil.TempFile(m.TempDir, "metron")
	Expect(err).NotTo(HaveOccurred())
	config := config.Config{
		Index: "42",
		Job:   "test-component",
		LoggregatorDropsondePort:  m.DropsondePort,
		IncomingUDPPort:           m.MetronPort,
		SharedSecret:              "shared_secret",
		EtcdUrls:                  m.EtcdRunner.NodeURLS(),
		EtcdMaxConcurrentRequests: 1,
		Zone:                             "z1",
		Deployment:                       "deployment-name",
		Protocols:                        m.Protocols,
		MetricBatchIntervalMilliseconds:  50,
		RuntimeStatsIntervalMilliseconds: 500,
		TCPBatchSizeBytes:                1024,
		TLSConfig: config.TLSConfig{
			CertFile: m.CertFile,
			KeyFile:  m.KeyFile,
			CAFile:   m.CAFile,
		},
	}
	configBytes, err := json.Marshal(config)
	_, err = cfgFile.Write(configBytes)
	Expect(err).NotTo(HaveOccurred())
	cfgFile.Close()

	command := exec.Command(m.Path, "--config", cfgFile.Name(), "--debug")
	command.Stdout = gexec.NewPrefixedWriter("[o][metron]", GinkgoWriter)
	command.Stderr = gexec.NewPrefixedWriter("[e][metron]", GinkgoWriter)
	m.Runner = ginkgomon.New(ginkgomon.Config{
		Name:          "metron",
		AnsiColorCode: "97m",
		StartCheck:    "metron started",
		Command:       command,
	})

	return m.Runner
}

func (m *MetronRunner) Start() ifrit.Process {
	runner := m.Configure()
	m.Process = ginkgomon.Invoke(runner)
	return m.Process
}

func (m *MetronRunner) Stop() {
	ginkgomon.Interrupt(m.Process, 2)
}

func (m *MetronRunner) MetronAddress() string {
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(m.MetronPort))
}

func (m *MetronRunner) LegacyMetronAddress() string {
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(m.LegacyPort))
}

func (m *MetronRunner) DropsondeAddress() string {
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(m.DropsondePort))
}
