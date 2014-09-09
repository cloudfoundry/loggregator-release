package channel_group_connector_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestChannelGroupConnector(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ChannelGroupConnector Suite")
}
