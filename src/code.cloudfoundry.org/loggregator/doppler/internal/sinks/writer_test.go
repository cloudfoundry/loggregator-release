package sinks_test

import (
	"net/url"
	"reflect"
	"time"

	"code.cloudfoundry.org/loggregator/doppler/internal/sinks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Writer", func() {

	It("returns an syslogWriter for syslog scheme", func() {
		outputUrl, _ := url.Parse("syslog://localhost:9999")
		w, err := sinks.NewWriter(outputUrl, "appId", "hostname", false, 1*time.Second, 0)
		Expect(err).ToNot(HaveOccurred())
		writerType := reflect.TypeOf(w).String()
		Expect(writerType).To(Equal("*sinks.syslogWriter"))
	})

	It("returns an tlsWriter for syslog-tls scheme", func() {
		outputUrl, _ := url.Parse("syslog-tls://localhost:9999")
		w, err := sinks.NewWriter(outputUrl, "appId", "hostname", false, 1*time.Second, 0)
		Expect(err).ToNot(HaveOccurred())
		writerType := reflect.TypeOf(w).String()
		Expect(writerType).To(Equal("*sinks.tlsWriter"))
	})

	It("returns an httpsWriter for https scheme", func() {
		outputUrl, _ := url.Parse("https://localhost:9999")
		w, err := sinks.NewWriter(outputUrl, "appId", "hostname", false, 1*time.Second, 0)
		Expect(err).ToNot(HaveOccurred())
		writerType := reflect.TypeOf(w).String()
		Expect(writerType).To(Equal("*sinks.httpsWriter"))
	})

	It("returns an error for invalid scheme", func() {
		outputUrl, _ := url.Parse("notValid://localhost:9999")
		w, err := sinks.NewWriter(outputUrl, "appId", "hostname", false, 1*time.Second, 0)
		Expect(err).To(HaveOccurred())
		Expect(w).To(BeNil())
	})
})
