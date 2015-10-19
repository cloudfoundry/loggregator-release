package clientpool_test

import (
	"math/rand"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
	"time"
)

func TestClientpool(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Clientpool Suite")
}

var _ = BeforeSuite(func() {
	rand.Seed(int64(time.Now().Nanosecond()))
})
