package config_test

import (
	"boshhmforwarder/config"

	"io/ioutil"
	"os"

	"boshhmforwarder/logging"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	Context("Good configuration", func() {
		It("reads the config from a file", func() {
			conf := config.Configuration("./assets/working.json")

			Expect(conf.IncomingPort).To(Equal(1234))
			Expect(conf.LogLevel).To(Equal(logging.DEBUG))
			Expect(conf.DebugPort).To(Equal(65434))
			Expect(conf.InfoPort).To(Equal(65433))
			Expect(conf.Syslog).To(Equal("vcap.boshhmforwarder"))
		})

		It("reads a config that will default the log level", func() {
			conf := config.Configuration("./assets/without_loglevel.json")
			Expect(conf.LogLevel).To(Equal(logging.INFO))
		})
	})

	It("panics if the file doesn't exist", func() {
		Expect(func() { config.Configuration("missing_file.json") }).To(Panic())
	})

	Context("Bad configuration", func() {
		var configFile *os.File

		BeforeEach(func() {
			var err error
			configFile, err = ioutil.TempFile("", "")
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			os.Remove(configFile.Name())
		})

		It("defaults the port to 0", func() {
			conf := config.Configuration("./assets/without_port.json")
			Expect(conf.IncomingPort).To(Equal(0))
		})

		It("errors if metronPort is not defined", func() {
			Expect(func() { config.Configuration("./assets/without_metron_port.json") }).Should(Panic())
		})

	})
})
