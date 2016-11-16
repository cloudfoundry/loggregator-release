package linter_test

import (
	"testing"
	"tools/linter"

	"github.com/apoydence/onpar"
	. "github.com/apoydence/onparginkgo"
)

func TestFileFilter(t *testing.T) {
	o := onpar.New()
	defer o.Run(t)

	o.Spec("it filters in regular files", func(t *testing.T) {
		mock := newMockFileInfo()
		mock.NameOutput.Ret0 <- "foo.go"
		Expect(t, linter.FileFilter(mock)).To(BeTrue())
	})

	o.Spec("it filters out test files", func(t *testing.T) {
		mock := newMockFileInfo()
		mock.NameOutput.Ret0 <- "foo_test.go"
		Expect(t, linter.FileFilter(mock)).To(BeFalse())
	})
}
