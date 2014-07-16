package integration_test

import (
	"encoding/json"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/localip"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"io/ioutil"
	"net/http"
	"os/exec"
)

var _ = Describe("Varz Endpoints", func() {
	var session *gexec.Session
	localIPAddress, _ := localip.LocalIP()

	BeforeSuite(func() {
		pathToMetronExecutable, err := gexec.Build("metron")
		Expect(err).ShouldNot(HaveOccurred())

		command := exec.Command(pathToMetronExecutable, "--configFile=fixtures/metron.json")

		session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).ShouldNot(HaveOccurred())

		// wait for server to be up
		Eventually(func() error {
			_, err := http.Get("http://" + localIPAddress + ":1234")
			return err
		}).ShouldNot(HaveOccurred())
	})

	AfterSuite(func() {
		session.Kill()
		gexec.CleanupBuildArtifacts()
	})

	Context("/varz", func() {
		It("shows basic metrics", func() {
			req, _ := http.NewRequest("GET", "http://"+localIPAddress+":1234/varz", nil)
			req.SetBasicAuth("admin", "admin")

			resp, _ := http.DefaultClient.Do(req)

			var message instrumentation.VarzMessage
			json.NewDecoder(resp.Body).Decode(&message)

			Expect(message.Name).To(Equal("MetronAgent"))
			Expect(message.Tags).To(HaveKeyWithValue("ip", localIPAddress))
			Expect(message.NumGoRoutines).To(BeNumerically(">", 0))
			Expect(message.NumCpus).To(BeNumerically(">", 0))
			Expect(message.MemoryStats.BytesAllocatedHeap).To(BeNumerically(">", 0))
		})
	})

	Context("/healthz", func() {
		It("is ok", func() {
			resp, _ := http.Get("http://" + localIPAddress + ":1234/healthz")

			bodyString, _ := ioutil.ReadAll(resp.Body)
			Expect(string(bodyString)).To(Equal("ok"))
		})
	})
})
