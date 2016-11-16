package main

import (
	"flag"
	"fmt"
	"go/build"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path"
	"tools/linter"

	"github.com/kisielk/gotool"
)

var (
	locksOnly   bool
	searchPaths searchPath
)

type searchPath struct {
	paths []string
}

func (s *searchPath) String() string {
	return fmt.Sprintf("%v", s.paths)
}

func (s *searchPath) Set(val string) error {
	s.paths = append(s.paths, val)
	return nil
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Var(&searchPaths, "path", "Required. The directory to search in, relative to the gopath. Multiple allowed. In form '___/___/ or ___/...'")
	flag.BoolVar(&locksOnly, "locks-only", true, "Only output errors that include locks")
}

func main() {
	var count int
	flag.Parse()
	if searchPaths.paths == nil {
		flag.Usage()
		os.Exit(1)
	}
	importPaths := gotool.ImportPaths(searchPaths.paths)
	d, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	for _, fpath := range importPaths {
		buildPkg, err := build.Import(fpath, d, 0)
		if err != nil {
			panic(err)
		}

		fset := token.NewFileSet()

		packages, err := parser.ParseDir(fset, buildPkg.Dir, linter.FileFilter, 0)
		if err != nil {
			panic(err)
		}
		if _, ok := packages[buildPkg.Name]; !ok {
			fmt.Printf("Skipping package: %s in path %s\n\n", buildPkg.Name, fpath)
			continue
		}

		astPkg := packages[buildPkg.Name]

		for fname, f := range astPkg.Files {
			funcs := linter.FuncDecls(f)
			problems := linter.CheckFuncs(funcs, fset, locksOnly)
			count += len(problems)
			for _, p := range problems {
				srcPath := path.Join(buildPkg.Dir, path.Base(fname))
				err := linter.PrintProblem(srcPath, p)
				if err != nil {
					log.Println("error in getting source: ", err)
				}
			}
		}
	}
	os.Exit(count)
}
