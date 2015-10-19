package integration_test

import (
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"strconv"

	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/pivotal-golang/localip"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"

	"github.com/onsi/ginkgo/config"
	"github.com/onsi/gomega/gexec"
)

func TestIntegrationTest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "IntegrationTest Suite")
}

var tmpdir string
var metronPath string
var port int
var incomingLegacyPort int
var incomingDropsondePort int
var dropsondePort int
var metronRunner *ginkgomon.Runner
var metronProcess ifrit.Process
var etcdRunner *etcdstorerunner.ETCDClusterRunner
var etcdAdapter storeadapter.StoreAdapter
var etcdPort int
var localIPAddress string

var _ = SynchronizedBeforeSuite(func() []byte {
	metronPath, err := gexec.Build("metron", "-race")
	Expect(err).ShouldNot(HaveOccurred())
	return []byte(metronPath)
}, func(path []byte) {
	metronPath = string(path)

	var err error
	tmpdir, err = ioutil.TempDir("", "metronintg")
	Expect(err).ShouldNot(HaveOccurred())

	localIPAddress, _ = localip.LocalIP()

	etcdPort = 5800 + (config.GinkgoConfig.ParallelNode)*10
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1, nil)
	etcdRunner.Start()
	etcdAdapter = etcdRunner.Adapter(nil)

	port = 51000 + config.GinkgoConfig.ParallelNode*10
	incomingLegacyPort = port
	incomingDropsondePort = port + 1
	dropsondePort = 3457 + config.GinkgoConfig.ParallelNode*10
})

var _ = SynchronizedAfterSuite(func() {
	if etcdRunner != nil {
		etcdRunner.Stop()
	}
	os.RemoveAll(tmpdir)
}, func() {
	gexec.CleanupBuildArtifacts()
})

var _ = BeforeEach(func() {
	etcdRunner.Reset()
})

func configureMetron(protocol string) {
	cfgFile, err := ioutil.TempFile(tmpdir, "metron")
	Expect(err).NotTo(HaveOccurred())
	_, err = cfgFile.WriteString(`
{
    "Index": 42,
    "Job": "test-component",
    "LegacyIncomingMessagesPort": ` + strconv.Itoa(incomingLegacyPort) + `,
    "DropsondeIncomingMessagesPort": ` + strconv.Itoa(incomingDropsondePort) + `,
    "SharedSecret": "shared_secret",
    "EtcdUrls"    : ["` + etcdRunner.NodeURLS()[0] + `"],
    "EtcdMaxConcurrentRequests": 1,
    "Zone": "z1",
    "Deployment": "deployment-name",
    "LoggregatorDropsondePort": ` + strconv.Itoa(dropsondePort) + `,
    "PreferredProtocol": "` + protocol + `"
}`)
	Expect(err).NotTo(HaveOccurred())
	cfgFile.Close()

	metronRunner = ginkgomon.New(ginkgomon.Config{
		Name:          "metron",
		AnsiColorCode: "97m",
		StartCheck:    "metron started",
		Command: exec.Command(
			metronPath,
			"--config", cfgFile.Name(),
			"--debug",
		),
	})
}

func startMetron(protocol string) {
	configureMetron(protocol)
	metronProcess = ginkgomon.Invoke(metronRunner)
}

func stopMetron() {
	ginkgomon.Interrupt(metronProcess)
}

func metronAddress() string {
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(incomingDropsondePort))
}

func legacymetronAddress() string {
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(incomingLegacyPort))
}

func dropsondeAddress() string {
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(dropsondePort))
}
