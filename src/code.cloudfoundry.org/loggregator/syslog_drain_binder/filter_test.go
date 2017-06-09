package main_test

import (
	syslog_drain_binder "code.cloudfoundry.org/loggregator/syslog_drain_binder"

	"code.cloudfoundry.org/loggregator/syslog_drain_binder/shared_types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("filter", func() {
	It("removes syslog drains that have the 2.0 drain url query arg", func() {
		app1 := shared_types.SyslogDrainBinding{
			Hostname:  "org.space.app1",
			DrainURLs: []string{"http://example.com"},
		}
		app2 := shared_types.SyslogDrainBinding{
			Hostname: "org.space.app2",
			DrainURLs: []string{
				"http://example.net?drain-version=2.0",
				"http://example.com",
			},
		}
		app3 := shared_types.SyslogDrainBinding{
			Hostname: "org.space.app3",
			DrainURLs: []string{
				"http://example.net?drain-version=2.0",
			},
		}
		input := shared_types.AllSyslogDrainBindings{
			"app1": app1,
			"app2": app2,
			"app3": app3,
		}
		expected := shared_types.AllSyslogDrainBindings{
			"app1": app1,
			"app2": shared_types.SyslogDrainBinding{
				Hostname: "org.space.app2",
				DrainURLs: []string{
					"http://example.com",
				},
			},
		}
		Expect(syslog_drain_binder.Filter(input)).To(Equal(expected))
	})

	It("ignores malformed syslog drains", func() {
		app1 := shared_types.SyslogDrainBinding{
			Hostname:  "org.space.app1",
			DrainURLs: []string{"http://example.com"},
		}
		app2 := shared_types.SyslogDrainBinding{
			Hostname: "org.space.app2",
			DrainURLs: []string{
				"://example.com?drain-version=2.0",
				"://example.net?drain-version=2.0",
			},
		}
		app3 := shared_types.SyslogDrainBinding{
			Hostname: "org.space.app3",
			DrainURLs: []string{
				"http://example.net?drain-version=2.0",
			},
		}
		input := shared_types.AllSyslogDrainBindings{
			"app1": app1,
			"app2": app2,
			"app3": app3,
		}
		expected := shared_types.AllSyslogDrainBindings{
			"app1": app1,
		}
		Expect(syslog_drain_binder.Filter(input)).To(Equal(expected))
	})
})
