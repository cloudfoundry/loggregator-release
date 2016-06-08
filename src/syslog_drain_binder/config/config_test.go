package config_test

import (
	"syslog_drain_binder/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ParseConfig", func() {
	Context("with a file that doesn't exist", func() {
		It("returns an error", func() {
			_, err := config.ParseConfig("fixtures/bad_file_path.json")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("open fixtures/bad_file_path.json: no such file or directory"))
		})
	})

	Context("with a file that contains bad json", func() {
		It("returns an error", func() {
			_, err := config.ParseConfig("fixtures/bad_json.json")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("invalid character '@' looking for beginning of value"))
		})
	})

	Context("without metron address", func() {
		It("returns an error", func() {
			_, err := config.ParseConfig("fixtures/no_metron_address.json")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Need Metron address (host:port)"))
		})
	})

	Context("with etcd max concurrent requests less than one", func() {
		It("returns an error", func() {
			_, err := config.ParseConfig("fixtures/bad_etcd_max_concurrent_requests.json")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Need EtcdMaxConcurrentRequests ≥ 1, received "))
		})
	})

	Context("with etcd tls enabled", func() {
		Context("with etcd tls certs/keys missing", func() {
			It("returns an error", func() {
				_, err := config.ParseConfig("fixtures/bad_etcd_tls.json")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Invalid etcd TLS client configuration"))
			})
		})
	})
})
