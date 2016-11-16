package linter

import (
	"os"
	"strings"
)

func FileFilter(f os.FileInfo) bool {
	return !strings.HasSuffix(f.Name(), "_test.go")
}
