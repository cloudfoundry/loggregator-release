package deaagent_test

import (
	"deaagent/domain"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

const wardenIdentifier = 56

var (
	tmpdir              string
	instancesJson       *os.File
	task1InputListener  net.Listener
	task1StderrListener net.Listener
	deaAgentSession     *gexec.Session
)

func TestDeaagent(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Deaagent Integration Suite")
}

var _ = BeforeSuite(func() {
	var err error
	tmpdir, err = ioutil.TempDir("", "deaagent")
	instancesJson, err := ioutil.TempFile(tmpdir, "instances.json")
	Expect(err).NotTo(HaveOccurred())
	fileContent := fmt.Sprintf(`{"instances": [{"state": "RUNNING", "application_id": "1234", "warden_job_id": %d, "warden_container_path":"%s", "instance_index": 3, "syslog_drain_urls": ["url1"]}]}`, wardenIdentifier, tmpdir)
	ioutil.WriteFile(instancesJson.Name(), []byte(fileContent), os.ModePerm)

	helperTask1 := &domain.Task{
		ApplicationId:       "1234",
		SourceName:          "App",
		WardenJobId:         wardenIdentifier,
		WardenContainerPath: tmpdir,
		Index:               3,
	}

	task1InputListener, task1StderrListener = setupTaskSockets(helperTask1)

	pathToDeaAgentExecutable, err := gexec.Build("deaagent/deaagent", "-race")
	Expect(err).ShouldNot(HaveOccurred())

	deaagentCommand := exec.Command(pathToDeaAgentExecutable, "--config=fixtures/deaagent.json", "--debug", "-instancesFile", instancesJson.Name())

	deaAgentSession, err = gexec.Start(deaagentCommand, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

})

var _ = AfterSuite(func() {
	task1InputListener.Close()
	task1StderrListener.Close()
	deaAgentSession.Kill().Wait(5)
	gexec.CleanupBuildArtifacts()

	os.RemoveAll(tmpdir)
})
