package eventlistener_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestEventListener(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "EventListener Suite")
}
