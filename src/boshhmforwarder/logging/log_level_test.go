package logging_test

import (
	"boshhmforwarder/logging"

	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type config struct {
	LogLevel logging.LogLevel
}

var _ = Describe("LogLevel", func() {
	It("correctly marshals the LogLevel", func() {
		logLevel := logging.INFO
		json, err := logLevel.MarshalJSON()

		Expect(err).ToNot(HaveOccurred())
		Expect(json).To(Equal([]byte(`"INFO"`)))
	})

	It("correctly unmarshals the LogLevel", func() {
		var logLevel logging.LogLevel
		err := logLevel.UnmarshalJSON([]byte(`"DEBUG"`))

		Expect(err).ToNot(HaveOccurred())
		Expect(logLevel).To(Equal(logging.DEBUG))
	})

	It("marshals to JSON inside a struct", func() {
		conf := config{
			LogLevel: logging.DEBUG,
		}

		json, err := json.Marshal(conf)
		Expect(err).ToNot(HaveOccurred())
		Expect(json).To(MatchJSON(`{"LogLevel": "DEBUG"}`))
	})
})
