package helpers

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"

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

func StartLogemitter(logfinPort string) (*gexec.Session, string) {
	port := testPort()
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
	port := testPort()
	os.Setenv("PORT", port)
	os.Setenv("EMITTER_INSTANCES", "1")
	if os.Getenv("COUNTER_INSTANCES") == "" {
		os.Setenv("COUNTER_INSTANCES", "1")
	}
	//PORT
	return startComponent(
		logfinExecutablePath,
		"logfin",
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

func CheckEndpoint(port, endpoint string, status int) bool {
	resp, _ := http.Get("http://localhost:" + port + "/" + endpoint)
	if resp != nil {
		return resp.StatusCode == status
	}

	return false
}

func testPort() string {
	add, _ := net.ResolveTCPAddr("tcp", ":0")
	l, _ := net.ListenTCP("tcp", add)
	defer l.Close()
	port := strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
	return port
}
