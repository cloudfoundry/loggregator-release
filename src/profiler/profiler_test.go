package profiler_test

import (
	"io/ioutil"
	"os"
	"profiler"
	"time"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/remyoudompheng/go-misc/pprof/parser"
)

var _ = Describe("Profiler", func() {
	var cpuProfilePath string
	var memProfilePath string
	var logger = loggertesthelper.Logger()

	BeforeEach(func() {
		tmpDir, err := ioutil.TempDir("/tmp", "tc-test")
		if err != nil {
			Fail(err.Error())
		}

		cpuProfilePath = tmpDir + "/cpu.pprof"
		memProfilePath = tmpDir + "/mem.pprof"
	})

	Describe("cpu profiling", func() {
		It("profiles cpu usage", func() {
			profiler := profiler.New(cpuProfilePath, "", 1*time.Millisecond, logger)
			profiler.Profile()

			profiler.Stop()

			f, err := os.Open(cpuProfilePath)
			Expect(err).NotTo(HaveOccurred())

			_, err = parser.NewCpuProfParser(f)
			Expect(err).NotTo(HaveOccurred())

			memProfile := profiler.GetMemProfileHandle()
			Expect(memProfile).To(BeNil())
		})

		It("panics when there is an error", func() {
			filePath := "/tmp/asdf/asdf.pprof"
			profiler := profiler.New(filePath, "", 1*time.Millisecond, logger)
			Expect(profiler.Profile).To(Panic())
		})
	})

	Describe("memory profiling", func() {
		It("profiles memory usage", func() {
			profiler := profiler.New("", memProfilePath, 1*time.Millisecond, logger)
			profiler.Profile()
			time.Sleep(2 * time.Millisecond)

			profiler.Stop()

			f, err := os.Open(memProfilePath)
			Expect(err).NotTo(HaveOccurred())

			_, err = parser.NewHeapProfParser(f)
			Expect(err).NotTo(HaveOccurred())
		})

		It("panics when there is an error", func() {
			filePath := "/tmp/asdf/asdf.pprof"
			profiler := profiler.New("", filePath, 1*time.Millisecond, logger)
			Expect(profiler.Profile).To(Panic())
		})
	})

	Describe("Stop", func() {
		It("closes the both profile file handles", func() {
			profiler := profiler.New(cpuProfilePath, memProfilePath, 1*time.Millisecond, logger)
			profiler.Profile()

			time.Sleep(2 * time.Millisecond)

			profiler.Stop()

			cpuProfile := profiler.GetCpuProfileHandle()
			Expect(cpuProfile).NotTo(BeNil())

			memProfile := profiler.GetMemProfileHandle()
			Expect(memProfile).NotTo(BeNil())

			var err error
			err = cpuProfile.Close()
			Expect(err).To(HaveOccurred())

			err = memProfile.Close()
			Expect(err).To(HaveOccurred())
		})

		It("stops mem profiling before closing the file handles", func() {
			profiler := profiler.New("", memProfilePath, 1*time.Millisecond, logger)
			profiler.Profile()
			time.Sleep(2 * time.Millisecond)

			profiler.Stop()

			fileContents := func() string {
				bytes, err := ioutil.ReadFile(memProfilePath)
				Expect(err).NotTo(HaveOccurred())

				return string(bytes)
			}

			initialContents := fileContents()
			Consistently(fileContents).Should(Equal(initialContents))

			cpuProfile := profiler.GetCpuProfileHandle()
			Expect(cpuProfile).To(BeNil())

			Expect(loggertesthelper.TestLoggerSink.LogContents()).NotTo(ContainSubstring("Error in profiler"))
		})
	})
})
