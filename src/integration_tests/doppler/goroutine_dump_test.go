package doppler_test

import (
	"syscall"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Goroutine Dump test", func() {
	It("emits the list of goroutines to STDOUT with each receipt of the USR1 signal", func() {
		dopplerSession.Signal(syscall.SIGUSR1)
		Eventually(dopplerSession.Out).Should(gbytes.Say(`goroutine \d+ \[running\]`))

		dopplerSession.Signal(syscall.SIGUSR1)
		Eventually(dopplerSession.Out).Should(gbytes.Say(`goroutine \d+ \[running\]`))
	})
})
