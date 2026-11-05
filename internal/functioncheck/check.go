// Package functioncheck checks production Go function size using syntax trees.
// It is build tooling only and is not imported by the desktop application.
package functioncheck

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

const MaxLines = 100

type Finding struct {
	Name  string
	Line  int
	Lines int
}

// CheckFile includes declarations and closures on every target platform. Tests
// remain covered by go test, rather than this production readability limit.
func CheckFile(name string, data []byte) ([]Finding, error) {
	if strings.HasSuffix(name, "_test.go") {
		return nil, nil
	}
	positions := token.NewFileSet()
	file, err := parser.ParseFile(positions, name, data, 0)
	if err != nil {
		return nil, err
	}
	var findings []Finding
	ast.Inspect(file, func(node ast.Node) bool {
		name, begin, end := functionRange(node)
		if begin.IsValid() {
			line := positions.Position(begin).Line
			length := positions.Position(end).Line - line + 1
			if length > MaxLines {
				findings = append(findings, Finding{Name: name, Line: line, Lines: length})
			}
		}
		return true
	})
	return findings, nil
}

func functionRange(node ast.Node) (string, token.Pos, token.Pos) {
	switch function := node.(type) {
	case *ast.FuncDecl:
		if function.Body != nil {
			return function.Name.Name, function.Pos(), function.End()
		}
	case *ast.FuncLit:
		return "<closure>", function.Pos(), function.End()
	}
	return "", token.NoPos, token.NoPos
}
