package runners

import (
	"io/ioutil"
	"net"
	"os/exec"
	"strconv"

	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/gomega"
)

type MetronRunner struct {
	Path    string
	TempDir string

	Protocol      string
	LegacyPort    int
	MetronPort    int
	DropsondePort int
	EtcdRunner    *etcdstorerunner.ETCDClusterRunner

	Runner  *ginkgomon.Runner
	Process ifrit.Process
}

func (m *MetronRunner) Configure() *ginkgomon.Runner {
	cfgFile, err := ioutil.TempFile(m.TempDir, "metron")
	Expect(err).NotTo(HaveOccurred())
	_, err = cfgFile.WriteString(`
{
    "Index": 42,
    "Job": "test-component",
    "LegacyIncomingMessagesPort": ` + strconv.Itoa(m.LegacyPort) + `,
    "DropsondeIncomingMessagesPort": ` + strconv.Itoa(m.MetronPort) + `,
    "SharedSecret": "shared_secret",
    "EtcdUrls"    : ["` + m.EtcdRunner.NodeURLS()[0] + `"],
    "EtcdMaxConcurrentRequests": 1,
    "Zone": "z1",
    "Deployment": "deployment-name",
    "LoggregatorDropsondePort": ` + strconv.Itoa(m.DropsondePort) + `,
    "PreferredProtocol": "` + m.Protocol + `",
    "MetricBatchIntervalSeconds": 1
}`)
	Expect(err).NotTo(HaveOccurred())
	cfgFile.Close()

	m.Runner = ginkgomon.New(ginkgomon.Config{
		Name:          "metron",
		AnsiColorCode: "97m",
		StartCheck:    "metron started",
		Command: exec.Command(
			m.Path,
			"--config", cfgFile.Name(),
			"--debug",
		),
	})

	return m.Runner
}

func (m *MetronRunner) Start() ifrit.Process {
	runner := m.Configure()
	m.Process = ginkgomon.Invoke(runner)
	return m.Process
}

func (m *MetronRunner) Stop() {
	ginkgomon.Interrupt(m.Process)
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
