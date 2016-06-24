package metron_test

import (
	"os"
	"strings"
	"sync"
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
	metronPath  string
	dopplerPath string
	etcdPath    string
)

var _ = SynchronizedBeforeSuite(func() []byte {
	// Note: There was discussion about building binaries globally for all test
	// packages. For now we are doing this in parallel once for all the tests in
	// this package.
	var mu sync.Mutex
	buildPaths := make([]string, 3)
	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		metronPath, err := gexec.Build("metron", "-race")
		Expect(err).ToNot(HaveOccurred())
		mu.Lock()
		defer mu.Unlock()
		buildPaths[0] = metronPath
	}()

	go func() {
		defer wg.Done()
		dopplerPath, err := gexec.Build("doppler", "-race")
		Expect(err).ToNot(HaveOccurred())
		mu.Lock()
		defer mu.Unlock()
		buildPaths[1] = dopplerPath
	}()

	go func() {
		defer wg.Done()
		etcdPath, err := gexec.Build("github.com/coreos/etcd", "-race")
		Expect(err).ToNot(HaveOccurred())
		mu.Lock()
		defer mu.Unlock()
		buildPaths[2] = etcdPath
	}()

	wg.Wait()
	return []byte(strings.Join(buildPaths, ":"))
}, func(buildPaths []byte) {
	paths := strings.Split(string(buildPaths), ":")
	metronPath = paths[0]
	dopplerPath = paths[1]
	etcdPath = paths[2]
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	os.Remove(etcdPath)
	os.Remove(dopplerPath)
	os.Remove(metronPath)
})
