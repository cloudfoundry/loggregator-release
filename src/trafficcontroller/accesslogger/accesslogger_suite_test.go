package accesslogger_test

import (
	"math/rand"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestAccesslogger(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Accesslogger Suite")
}

var _ = BeforeSuite(func() {
	rand.Seed(time.Now().UnixNano())
})
