package config_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"metron/config"
	"os"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Parse", func() {
	It("sets defaults", func() {
		cfg, err := config.Parse(strings.NewReader(`{}`))
		Expect(err).ToNot(HaveOccurred())

		Expect(cfg).To(Equal(&config.Config{
			TCPBatchIntervalMilliseconds:     100,
			TCPBatchSizeBytes:                10240,
			MetricBatchIntervalMilliseconds:  5000,
			RuntimeStatsIntervalMilliseconds: 15000,
			Protocols:                        config.Protocols{"udp": struct{}{}},
		}))
	})

	Context("with valid protocols", func() {
		DescribeTable("doesn't return error", func(proto string) {
			json := fmt.Sprintf(`{"Protocols": ["%s"]}`, proto)
			_, err := config.Parse(strings.NewReader(json))
			Expect(err).ToNot(HaveOccurred())
		},
			Entry("udp", "udp"),
			Entry("tcp", "tcp"),
			Entry("tls", "tls"),
		)
	})

	Context("with an invalid protocol", func() {
		It("returns error", func() {
			json := `{"Protocols": ["NOT A REAL PROTOCOL"]}`
			_, err := config.Parse(strings.NewReader(json))

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Invalid protocol: NOT A REAL PROTOCOL"))
		})
	})

	Context("with empty Protocols key", func() {
		It("returns error", func() {
			json := `{"Protocols": []}`
			_, err := config.Parse(strings.NewReader(json))

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Metron cannot start without protocols"))
		})
	})

	Context("with a bad etcd tls config", func() {
		It("errors out", func() {
			json := `{"EtcdRequireTLS": true}`
			_, err := config.Parse(strings.NewReader(json))

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("invalid etcd TLS client configuration"))
		})
	})
})

var _ = Describe("ParseConfig", func() {
	It("returns proper config", func() {
		json := `{
			"Index": "0",
			"Job": "job-name",
			"Zone": "z1",
			"Deployment": "deployment-name",

			"EtcdUrls"    : ["http://127.0.0.1:4001"],
			"EtcdMaxConcurrentRequests": 1,
			"EtcdQueryIntervalMilliseconds": 100,

			"SharedSecret": "shared_secret",

			"IncomingUDPPort": 51161,
			"LoggregatorDropsondePort": 3457,
			"Syslog": "syslog.namespace",
			"MetricBatchIntervalMilliseconds": 20,
			"RuntimeStatsIntervalMilliseconds": 15,
			"TCPBatchSizeBytes": 1024,
			"TCPBatchIntervalMilliseconds": 10,
			"Protocols": ["tls", "udp"],
			"TLSConfig": {
				"KeyFile": "./fixtures/client.key",
				"CertFile": "./fixtures/client.crt",
				"CAFile": "./fixtures/ca.crt"
			}
		}`
		r := strings.NewReader(json)
		path, cleanup := tmpFile(json)
		defer cleanup()

		c1, err := config.ParseConfig(path)
		Expect(err).ToNot(HaveOccurred())
		c2, err := config.Parse(r)
		Expect(err).ToNot(HaveOccurred())
		Expect(c1).To(Equal(c2))
	})

	Context("with a non-existent file", func() {
		It("returns an error", func() {
			configFile := "invalid-path-24570yjomohq4uwrth.json"
			_, err := config.ParseConfig(configFile)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("with PPROFPort", func() {
		It("uses specified value", func() {
			configFile := "./fixtures/metron.json"

			c, err := config.ParseConfig(configFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(c.PPROFPort).To(Equal(uint32(666)))
		})
	})
})

var _ = Describe("Protocols", func() {
	Describe("UnmarshalJSON", func() {
		It("dedupes protocols", func() {
			jsn := `["tcp", "tcp", "tcp"]`
			var p config.Protocols
			Expect(json.Unmarshal([]byte(jsn), &p)).To(Succeed())
			Expect(p.Strings()).To(ConsistOf(
				"tcp",
			))
		})
	})
})

var _ = Describe("MarshalJSON", func() {
	It("marshals to an unmarshallable value", func() {
		cfg := config.Protocols{"tcp": struct{}{}}
		v, err := json.Marshal(cfg)
		Expect(err).ToNot(HaveOccurred())
		var result config.Protocols
		Expect(json.Unmarshal(v, &result)).To(Succeed())
		Expect(result).To(Equal(cfg))
	})
})

func tmpFile(s string) (string, func()) {
	d, err := ioutil.TempDir("", "")
	Expect(err).ToNot(HaveOccurred())
	f, err := ioutil.TempFile(d, "")
	Expect(err).ToNot(HaveOccurred())
	defer f.Close()
	fmt.Fprint(f, s)
	return f.Name(), func() {
		os.RemoveAll(d)
	}
}
