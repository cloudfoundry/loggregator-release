package linter_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"tools/linter"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("nodes", func() {
	Context("FunDecls", func() {
		It("returns funcs", func() {
			src := `
			package foo

			var Foo int
			func Bar() {}
		`
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "foo.go", src, 0)
			Expect(err).To(Not(HaveOccurred()))

			funcs := linter.FuncDecls(f)
			Expect(funcs).To(HaveLen(1))
			Expect(funcs[0].Name.Name).To(Equal("Bar"))
		})
	})

	Context("CheckFuncs", func() {
		Context("with locked only flag disabled", func() {
			It("detects bare message sends", func() {
				src := `
				func BadSend() {
					send <- true
				}
			`
				funcs, fset := parse(src)
				Expect(linter.CheckFuncs(funcs, fset, false)).To(ConsistOf(
					linter.Problem{
						Kind: "sendChannel-withoutSelect",
						Position: token.Position{
							Filename: "foo.go",
							Offset:   40,
							Line:     5,
							Column:   6,
						},
					},
				))
			})

			It("detects bare message receives", func() {
				src := `
				func BadReceive() {
					<-recv
				}
			`
				funcs, fset := parse(src)
				Expect(linter.CheckFuncs(funcs, fset, false)).To(ConsistOf(
					linter.Problem{
						Kind: "receiveChannel-withoutSelect",
						Position: token.Position{
							Filename: "foo.go",
							Offset:   43,
							Line:     5,
							Column:   6,
						},
					},
				))
			})

			It("detects selects without defaults", func() {
				src := `
				func BadSelect() {
					select {}
				}
			`
				funcs, fset := parse(src)
				Expect(linter.CheckFuncs(funcs, fset, false)).To(ConsistOf(
					linter.Problem{
						Kind: "selectWithoutDefault",
						Position: token.Position{
							Filename: "foo.go",
							Offset:   42,
							Line:     5,
							Column:   6,
						},
					},
				))
			})

			It("detects channel sends with locks", func() {
				src := `
				func BadLockSend() {
					mu.Lock()
					defer mu.Unlock()
					send <- foo
				}
			`
				funcs, fset := parse(src)
				Expect(linter.CheckFuncs(funcs, fset, false)).To(ConsistOf(
					linter.Problem{
						Kind: "sendChannel-withoutSelect-lock",
						Position: token.Position{
							Filename: "foo.go",
							Offset:   82,
							Line:     7,
							Column:   6,
						},
					},
				))
			})

			It("detects channel receive with locks", func() {
				src := `
				func BadLockReceive() {
					mu.Lock()
					defer mu.Unlock()
					<-foo
				}
			`
				funcs, fset := parse(src)
				Expect(linter.CheckFuncs(funcs, fset, false)).To(ConsistOf(
					linter.Problem{
						Kind: "receiveChannel-withoutSelect-lock",
						Position: token.Position{
							Filename: "foo.go",
							Offset:   85,
							Line:     7,
							Column:   6,
						},
					},
				))
			})

			It("detects selects with locks", func() {
				src := `
				func BadLockSelect() {
					mu.Lock()
					defer mu.Unlock()
					select {}
				}
			`
				funcs, fset := parse(src)
				Expect(linter.CheckFuncs(funcs, fset, false)).To(ConsistOf(
					linter.Problem{
						Kind: "selectWithoutDefault-lock",
						Position: token.Position{
							Filename: "foo.go",
							Offset:   84,
							Line:     7,
							Column:   6,
						},
					},
				))
			})

			It("ignores selects with default case", func() {
				src := `
				func GoodSelect() {
					select {
					default:
					}
				}
			`
				funcs, fset := parse(src)
				Expect(linter.CheckFuncs(funcs, fset, false)).To(BeEmpty())
			})

			It("ignores selects with locks with default case", func() {
				src := `
				func GoodLock() {
					mu.Lock()
					defer mu.Unlock()
					select {
					default:
					}
				}
			`
				funcs, fset := parse(src)
				Expect(linter.CheckFuncs(funcs, fset, false)).To(BeEmpty())
			})
		})

		Context("with locked only flag enabled", func() {
			It("ignores funcs that doesn't have locks", func() {
				src := `
				func BadSend() {
					send <- true
				}

				func BadReceive() {
					<-recv
				}

				func BadSelect() {
					select {}
				}
			`
				funcs, fset := parse(src)
				Expect(linter.CheckFuncs(funcs, fset, true)).To(BeEmpty())
			})

			It("detects channel sends with locks", func() {
				src := `
				func BadLockSend() {
					mu.Lock()
					defer mu.Unlock()
					send <- foo
				}
			`
				funcs, fset := parse(src)
				Expect(linter.CheckFuncs(funcs, fset, true)).To(ConsistOf(
					linter.Problem{
						Kind: "sendChannel-withoutSelect-lock",
						Position: token.Position{
							Filename: "foo.go",
							Offset:   82,
							Line:     7,
							Column:   6,
						},
					},
				))
			})

			It("detects channel receive with locks", func() {
				src := `
				func BadLockReceive() {
					mu.Lock()
					defer mu.Unlock()
					<-foo
				}
			`
				funcs, fset := parse(src)
				Expect(linter.CheckFuncs(funcs, fset, true)).To(ConsistOf(
					linter.Problem{
						Kind: "receiveChannel-withoutSelect-lock",
						Position: token.Position{
							Filename: "foo.go",
							Offset:   85,
							Line:     7,
							Column:   6,
						},
					},
				))
			})

			It("detects selects with locks", func() {
				src := `
				func BadLockSelect() {
					mu.Lock()
					defer mu.Unlock()
					select {}
				}
			`
				funcs, fset := parse(src)
				Expect(linter.CheckFuncs(funcs, fset, true)).To(ConsistOf(
					linter.Problem{
						Kind: "selectWithoutDefault-lock",
						Position: token.Position{
							Filename: "foo.go",
							Offset:   84,
							Line:     7,
							Column:   6,
						},
					},
				))
			})

			It("ignores selects with default case", func() {
				src := `
				func GoodSelect() {
					select {
					default:
					}
				}
			`
				funcs, fset := parse(src)
				Expect(linter.CheckFuncs(funcs, fset, true)).To(BeEmpty())
			})

			It("ignores selects with locks with default case", func() {
				src := `
				func GoodLock() {
					mu.Lock()
					defer mu.Unlock()
					select {
					default:
					}
				}
			`
				funcs, fset := parse(src)
				Expect(linter.CheckFuncs(funcs, fset, true)).To(BeEmpty())
			})
		})
	})
})

func parse(src string) ([]*ast.FuncDecl, *token.FileSet) {
	pkgSrc := "package foo\n\n" + src
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "foo.go", pkgSrc, 0)
	Expect(err).To(Not(HaveOccurred()))
	return linter.FuncDecls(f), fset
}
