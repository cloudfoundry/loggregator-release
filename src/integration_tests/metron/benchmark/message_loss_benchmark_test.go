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
		buf := bytes.NewBuffer(nil)
		benchmarkSession, err := gexec.Start(command, buf, buf)
		Expect(err).ToNot(HaveOccurred())
		Eventually(benchmarkSession, 15).Should(gexec.Exit())
		out := buf.String()
		Expect(out).To(ContainSubstring("PercentLoss"))
		lines := strings.Split(out, "\n")
		Expect(lines).To(HaveLen(3))
		values := strings.Split(lines[1], ", ")
		Expect(values).To(HaveLen(3))

		percentLoss, err := strconv.ParseFloat(values[2], 64)
		Expect(err).ToNot(HaveOccurred())
		Expect(percentLoss).To(BeNumerically("<", 1.0))
		b.RecordValue("message loss (percent)", percentLoss)

		messageThroughput, err := strconv.ParseFloat(values[0], 64)
		Expect(err).ToNot(HaveOccurred())
		b.RecordValue("message throughput", messageThroughput)
	}, 3)
})
