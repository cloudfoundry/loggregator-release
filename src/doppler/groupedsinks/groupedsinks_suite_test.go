package groupedsinks_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestGroupedsinks(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Groupedsinks Suite")
}
