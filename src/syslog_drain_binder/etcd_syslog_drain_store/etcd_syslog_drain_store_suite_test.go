package etcd_syslog_drain_store_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestSyslogDrainStore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "EtcdSyslogDrainStore Suite")
}
