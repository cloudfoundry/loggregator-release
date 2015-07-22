package benchmark_test

import (
	"bytes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"os/exec"
	"strconv"
	"strings"
)

var _ = Describe("MessageLossBenchmark", func() {
	Measure("message loss and throughput", func(b Benchmarker) {
		pathToMetronBenchmarkExec, err := gexec.Build("tools/metronbenchmark")
		Expect(err).NotTo(HaveOccurred())

		command := exec.Command(pathToMetronBenchmarkExec, "-writeRate", "5000", "-interval",
			"10s", "-stopAfter", "11s")
		outBuffer := bytes.NewBuffer(nil)
		errBuffer := bytes.NewBuffer(nil)
		benchmarkSession, err := gexec.Start(command, outBuffer, errBuffer)
		Expect(err).ToNot(HaveOccurred())
		Eventually(benchmarkSession, 15).Should(gexec.Exit())
		out := outBuffer.String()
		Expect(out).To(ContainSubstring("PercentLoss"))
		lines := strings.Split(out, "\n")
		Expect(lines).To(HaveLen(4))
		values := strings.Split(lines[1], ", ")
		Expect(values).To(HaveLen(3))

		value := strings.Split(values[2], "%")[0]
		percentLoss, err := strconv.ParseFloat(value, 64)
		Expect(err).ToNot(HaveOccurred())
		Expect(percentLoss).To(BeNumerically("<", 1.0))
		b.RecordValue("message loss (percent)", percentLoss)

		messageThroughput, err := strconv.ParseFloat(values[0], 64)
		Expect(err).ToNot(HaveOccurred())
		b.RecordValue("message throughput", messageThroughput)
	}, 3)

	Measure("message loss and throughput with legacy and dropsonde messages", func(b Benchmarker) {
		pathToMetronBenchmarkExec, err := gexec.Build("tools/legacymetronbenchmark")
		Expect(err).NotTo(HaveOccurred())

		command := exec.Command(pathToMetronBenchmarkExec, "-writeRate", "5000", "-interval",
			"10s", "-stopAfter", "11s")
		outBuffer := bytes.NewBuffer(nil)
		errBuffer := bytes.NewBuffer(nil)
		benchmarkSession, err := gexec.Start(command, outBuffer, errBuffer)
		Expect(err).ToNot(HaveOccurred())
		Eventually(benchmarkSession, 15).Should(gexec.Exit())
		out := outBuffer.String()
		Expect(out).To(ContainSubstring("PercentLoss"))
		lines := strings.Split(out, "\n")
		Expect(lines).To(HaveLen(4))
		values := strings.Split(lines[1], ", ")
		Expect(values).To(HaveLen(4))

		value := strings.Split(values[2], "%")[0]
		percentLoss, err := strconv.ParseFloat(value, 64)
		Expect(err).ToNot(HaveOccurred())
		Expect(percentLoss).To(BeNumerically("<", 1.0))
		b.RecordValue("message loss (percent)", percentLoss)

		messageThroughput, err := strconv.ParseFloat(values[0], 64)
		Expect(err).ToNot(HaveOccurred())
		b.RecordValue("message throughput", messageThroughput)
	}, 3)
})
