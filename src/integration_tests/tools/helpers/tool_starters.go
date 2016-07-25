package helpers

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var (
	logemitterExecutablePath string
	logfinExecutablePath     string
	logcounterExecutablePath string
)

func BuildLogfin() {
	logfinExecutablePath = buildComponent("tools/logfin")
}

func BuildLogemitter() {
	logemitterExecutablePath = buildComponent("tools/logemitter")
}

func BuildLogcounter() {
	logemitterExecutablePath = buildComponent("tools/logcounter")
}

func StartLogemitter(logfinPort string) (*gexec.Session, string) {
	port := "8080"
	os.Setenv("PORT", port)
	os.Setenv("RATE", "100")
	os.Setenv("TIME", ".5s")
	os.Setenv("LOGFIN_URL", "http://localhost:"+logfinPort)
	//PORT
	return startComponent(
		logemitterExecutablePath,
		"logemitter",
		34,
	), port
}

func StartLogfin() (*gexec.Session, string) {
	port := "8082"
	os.Setenv("PORT", port)
	os.Setenv("INSTANCES", "1")
	//PORT
	return startComponent(
		logfinExecutablePath,
		"logfin",
		34,
	), port
}

func StartLogcounter(uaaPort string) (*gexec.Session, string) {
	port := "8083"
	os.Setenv("PORT", port)
	os.Setenv("DOPPLER_URL", "127.0.0.1:1235") // this is a hardcoded address in fake_doppler
	//PORT
	return startComponent(
		logemitterExecutablePath,
		"logemitter",
		34,
	), port
}

func buildComponent(componentName string) (pathToComponent string) {
	var err error
	pathToComponent, err = gexec.Build(componentName)
	Expect(err).ToNot(HaveOccurred())
	return pathToComponent
}

func startComponent(path string, shortName string, colorCode uint64, arg ...string) *gexec.Session {
	var session *gexec.Session
	var err error
	startCommand := exec.Command(path, arg...)
	session, err = gexec.Start(
		startCommand,
		gexec.NewPrefixedWriter(fmt.Sprintf("\x1b[32m[o]\x1b[%dm[%s]\x1b[0m ", colorCode, shortName), GinkgoWriter),
		gexec.NewPrefixedWriter(fmt.Sprintf("\x1b[91m[e]\x1b[%dm[%s]\x1b[0m ", colorCode, shortName), GinkgoWriter))
	Expect(err).ToNot(HaveOccurred())
	return session
}

func CheckEndpoint(port, endpoint string) bool {
	resp, _ := http.Get("http://localhost:" + port + "/" + endpoint)
	if resp != nil {
		return resp.StatusCode == http.StatusOK
	}

	return false
}
