package firehose_group_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestFirehoseGroup(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "FirehoseGroup Suite")
}
