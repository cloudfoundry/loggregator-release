package linter_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
	"tools/linter"

	"github.com/apoydence/onpar"
	. "github.com/apoydence/onparginkgo"
)

func TestFuncDecls(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.Spec("it returns funcs", func(t *testing.T) {
		src := `
			package foo

			var Foo int
			func Bar() {}
		`
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "foo.go", src, 0)
		Expect(t, err).To(Not(HaveOccurred()))

		funcs := linter.FuncDecls(f)
		Expect(t, funcs).To(HaveLen(1))
		Expect(t, funcs[0].Name.Name).To(Equal("Bar"))
	})
}

func TestCheckFuncs(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.Group("with locked only flag disabled", func() {
		o.Spec("it detects bare message sends", func(t *testing.T) {
			src := `
				func BadSend() {
					send <- true
				}
			`
			funcs, fset := parse(t, src)
			Expect(t, linter.CheckFuncs(funcs, fset, false)).To(ConsistOf(
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

		o.Spec("it detects bare message receives", func(t *testing.T) {
			src := `
				func BadReceive() {
					<-recv
				}
			`
			funcs, fset := parse(t, src)
			Expect(t, linter.CheckFuncs(funcs, fset, false)).To(ConsistOf(
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

		o.Spec("it detects selects without defaults", func(t *testing.T) {
			src := `
				func BadSelect() {
					select {}
				}
			`
			funcs, fset := parse(t, src)
			Expect(t, linter.CheckFuncs(funcs, fset, false)).To(ConsistOf(
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

		o.Spec("it detects channel sends with locks", func(t *testing.T) {
			src := `
				func BadLockSend() {
					mu.Lock()
					defer mu.Unlock()
					send <- foo
				}
			`
			funcs, fset := parse(t, src)
			Expect(t, linter.CheckFuncs(funcs, fset, false)).To(ConsistOf(
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

		o.Spec("it detects channel receive with locks", func(t *testing.T) {
			src := `
				func BadLockReceive() {
					mu.Lock()
					defer mu.Unlock()
					<-foo
				}
			`
			funcs, fset := parse(t, src)
			Expect(t, linter.CheckFuncs(funcs, fset, false)).To(ConsistOf(
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

		o.Spec("it detects selects with locks", func(t *testing.T) {
			src := `
				func BadLockSelect() {
					mu.Lock()
					defer mu.Unlock()
					select {}
				}
			`
			funcs, fset := parse(t, src)
			Expect(t, linter.CheckFuncs(funcs, fset, false)).To(ConsistOf(
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

		o.Spec("it ignores selects with default case", func(t *testing.T) {
			src := `
				func GoodSelect() {
					select {
					default:
					}
				}
			`
			funcs, fset := parse(t, src)
			Expect(t, linter.CheckFuncs(funcs, fset, false)).To(BeEmpty())
		})

		o.Spec("it ignores selects with locks with default case", func(t *testing.T) {
			src := `
				func GoodLock() {
					mu.Lock()
					defer mu.Unlock()
					select {
					default:
					}
				}
			`
			funcs, fset := parse(t, src)
			Expect(t, linter.CheckFuncs(funcs, fset, false)).To(BeEmpty())
		})
	})

	o.Group("with locked only flag enabled", func() {
		o.Spec("it ignores funcs that doesn't have locks", func(t *testing.T) {
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
			funcs, fset := parse(t, src)
			Expect(t, linter.CheckFuncs(funcs, fset, true)).To(BeEmpty())
		})

		o.Spec("it detects channel sends with locks", func(t *testing.T) {
			src := `
				func BadLockSend() {
					mu.Lock()
					defer mu.Unlock()
					send <- foo
				}
			`
			funcs, fset := parse(t, src)
			Expect(t, linter.CheckFuncs(funcs, fset, true)).To(ConsistOf(
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

		o.Spec("it detects channel receive with locks", func(t *testing.T) {
			src := `
				func BadLockReceive() {
					mu.Lock()
					defer mu.Unlock()
					<-foo
				}
			`
			funcs, fset := parse(t, src)
			Expect(t, linter.CheckFuncs(funcs, fset, true)).To(ConsistOf(
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

		o.Spec("it detects selects with locks", func(t *testing.T) {
			src := `
				func BadLockSelect() {
					mu.Lock()
					defer mu.Unlock()
					select {}
				}
			`
			funcs, fset := parse(t, src)
			Expect(t, linter.CheckFuncs(funcs, fset, true)).To(ConsistOf(
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

		o.Spec("it ignores selects with default case", func(t *testing.T) {
			src := `
				func GoodSelect() {
					select {
					default:
					}
				}
			`
			funcs, fset := parse(t, src)
			Expect(t, linter.CheckFuncs(funcs, fset, true)).To(BeEmpty())
		})

		o.Spec("it ignores selects with locks with default case", func(t *testing.T) {
			src := `
				func GoodLock() {
					mu.Lock()
					defer mu.Unlock()
					select {
					default:
					}
				}
			`
			funcs, fset := parse(t, src)
			Expect(t, linter.CheckFuncs(funcs, fset, true)).To(BeEmpty())
		})
	})
}

func parse(t *testing.T, src string) ([]*ast.FuncDecl, *token.FileSet) {
	pkgSrc := "package foo\n\n" + src
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "foo.go", pkgSrc, 0)
	Expect(t, err).To(Not(HaveOccurred()))
	return linter.FuncDecls(f), fset
}
