package metronbenchmark_test

import (
	"bytes"
	"os/exec"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gexec"
)

var _ = Describe("MetronBenchmark tool", func() {
	BeforeEach(func() {
		var err error

		command := exec.Command(pathToMetronExecutable, "--config=fixtures/metron.json")
		metronSession, err = gexec.Start(command, gexec.NewPrefixedWriter("[o][metron]", GinkgoWriter), gexec.NewPrefixedWriter("[e][metron]", GinkgoWriter))
		Expect(err).ToNot(HaveOccurred())

		// TODO: figure out a better way to let metron finish starting up.
		time.Sleep(500 * time.Millisecond)
	})

	AfterEach(func() {
		metronSession.Kill().Wait()
	})

	DescribeTable("message rates",
		func(messagesPerSecond, threshold int) {
			command := exec.Command(pathToMetronBenchmarkExec, "-writeRate", strconv.Itoa(messagesPerSecond), "-interval",
				"5s", "-stopAfter", "6s")
			outBuffer := &bytes.Buffer{}
			errBuffer := &bytes.Buffer{}
			benchmarkSession, err := gexec.Start(command, outBuffer, errBuffer)
			Expect(err).ToNot(HaveOccurred())
			Eventually(benchmarkSession, 8).Should(gexec.Exit())
			out := outBuffer.String()
			Expect(out).To(ContainSubstring("Rate"))
			lines := strings.Split(out, "\n")
			Expect(lines).To(HaveLen(4))
			values := strings.Split(lines[1], ", ")
			Expect(values).To(HaveLen(5))

			value := strings.TrimSuffix(strings.TrimSpace(values[3]), "/s")
			rate, err := strconv.ParseFloat(value, 64)
			Expect(err).ToNot(HaveOccurred())
			Expect(rate).To(BeNumerically("~", messagesPerSecond, threshold))
		},
		Entry("4000 messages per second", 4000, 200),
		Entry("7000 messages per second", 7000, 350),
		Entry("10000 messages per second", 10000, 500),
		Entry("13000 messages per second", 13000, 650),
		Entry("17000 messages per second", 17000, 850),
	)
})
