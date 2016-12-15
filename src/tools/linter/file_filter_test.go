package linter_test

import (
	"tools/linter"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("File Filter", func() {
	It("filters in regular files", func() {
		mock := newMockFileInfo()
		mock.NameOutput.Ret0 <- "foo.go"
		Expect(linter.FileFilter(mock)).To(BeTrue())
	})

	It("filters out test files", func() {
		mock := newMockFileInfo()
		mock.NameOutput.Ret0 <- "foo_test.go"
		Expect(linter.FileFilter(mock)).To(BeFalse())
	})
})
