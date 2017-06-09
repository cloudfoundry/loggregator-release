package doppler_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Doppler Announcer", func() {
	It("advertises udp and ws endpoints", func() {
		node, err := etcdAdapter.Get("/doppler/meta/z1/doppler_z1/0")
		Expect(err).ToNot(HaveOccurred())

		expectedJSON := fmt.Sprintf(
			`{"version": 1, "endpoints":["udp://%[1]s:8765", "ws://%[1]s:4567"]}`,
			localIPAddress)

		Expect(node.Value).To(MatchJSON(expectedJSON))
	})
})
