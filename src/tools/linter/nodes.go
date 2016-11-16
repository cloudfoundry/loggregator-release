package linter

import (
	"go/ast"
	"go/token"
	"strings"
)

// Problem represents a problem found in the source code.
type Problem struct {
	Kind string
	token.Position
}

// CheckFuncs returns where there are problems given a set of potentially bad
// function declarations.
func CheckFuncs(funcs []*ast.FuncDecl, fset *token.FileSet, locksOnly bool) []Problem {
	p := &problemContainer{}
	for _, fd := range funcs {
		var lockAquired bool
		c := context{
			lockAquired:      &lockAquired,
			problemContainer: p,
			fset:             fset,
		}
		ast.Walk(c, fd)
	}

	ret := p.problems
	if locksOnly {
		ret = make([]Problem, 0)
		for _, problem := range p.problems {
			if strings.HasSuffix(problem.Kind, "-lock") {
				ret = append(ret, problem)
			}
		}
	}
	return ret
}

type context struct {
	lockAquired *bool
	inSelect    bool
	fset        *token.FileSet
	*problemContainer
}

type problemContainer struct {
	problems []Problem
}

func (c context) Visit(n ast.Node) ast.Visitor {
	switch m := n.(type) {
	case *ast.CallExpr:
		if isLock(m) {
			*c.lockAquired = true
		}
	case *ast.SelectStmt:
		c.inSelect = true
		if !hasDefault(m) {
			p := Problem{
				Kind:     "selectWithoutDefault",
				Position: c.fset.Position(m.Select),
			}

			if *c.lockAquired {
				p.Kind = "selectWithoutDefault-lock"
			}

			c.problems = append(c.problems, p)
			return c
		}
	case *ast.SendStmt:
		if !c.inSelect {
			p := Problem{
				Kind:     "sendChannel-withoutSelect",
				Position: c.fset.Position(m.Pos()),
			}

			if *c.lockAquired {
				p.Kind = "sendChannel-withoutSelect-lock"
			}

			c.problems = append(c.problems, p)
			return c
		}
	case *ast.UnaryExpr:
		if m.Op != token.ARROW {
			break
		}

		if !c.inSelect {
			p := Problem{
				Kind:     "receiveChannel-withoutSelect",
				Position: c.fset.Position(m.Pos()),
			}

			if *c.lockAquired {
				p.Kind = "receiveChannel-withoutSelect-lock"
			}

			c.problems = append(c.problems, p)
			return c
		}
	}
	return c
}

func isLock(s *ast.CallExpr) bool {
	se, ok := s.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return se.Sel.Name == "Lock"
}

func hasDefault(s *ast.SelectStmt) bool {
	for _, c := range s.Body.List {
		cc, ok := c.(*ast.CommClause)
		if !ok {
			continue
		}
		if cc.Comm == nil {
			return true
		}
	}
	return false
}

// FuncDecls returns all the function declarations in a file.
func FuncDecls(f *ast.File) []*ast.FuncDecl {
	result := make([]*ast.FuncDecl, 0)
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		result = append(result, fd)
	}
	return result
}
