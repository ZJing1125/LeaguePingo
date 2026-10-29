package main

import (
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"

	"leaguepingo/internal/apperrors"
)

func TestWailsErrorEnvelopeDoesNotLeakOriginalMessage(t *testing.T) {
	// Bindings log through Wails' application singleton. Do not Run it or
	// create windows, a League source, shortcuts or an assistant engine.
	_ = application.New(application.Options{Name: "LeaguePingo error envelope test"})
	s := &AppService{configureShortcut: func(string) error { return errors.New("private-player#DEMO https://host/?token=secret HTTP 503") }}
	bindings := application.NewBindings(nil, nil)
	if err := bindings.Add(application.NewServiceWithOptions(s, application.ServiceOptions{MarshalError: apperrors.Marshal})); err != nil {
		t.Fatal(err)
	}
	method := bindings.Get(&application.CallOptions{MethodName: "leaguepingo.AppService.ConfigureShortcut"})
	if method == nil {
		t.Fatal("service binding missing")
	}
	_, err := method.Call(context.Background(), []json.RawMessage{json.RawMessage(`"Ctrl+Shift+P"`)})
	var call *application.CallError
	if !errors.As(err, &call) {
		t.Fatalf("missing Wails error: %v", err)
	}
	data, _ := json.Marshal(call)
	var envelope struct {
		Message string                 `json:"message"`
		Cause   apperrors.Presentation `json:"cause"`
	}
	if json.Unmarshal(data, &envelope) != nil || envelope.Message != apperrors.Message(apperrors.Unexpected) || envelope.Cause.Code != apperrors.Unexpected || envelope.Cause.Message != envelope.Message {
		t.Fatalf("unsafe/inconsistent envelope: %s", data)
	}
	for _, secret := range []string{"private-player", "https://", "token", "secret", "HTTP", "503"} {
		if strings.Contains(string(data), secret) {
			t.Fatal("source text escaped into Wails envelope")
		}
	}
}

// Wails always joins returned error.Error() into its top-level message, even
// with MarshalError configured. Enforce the guard for future service methods.
func TestEveryServiceErrorReturnHasPublicBoundary(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		tree, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range tree.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || !fn.Name.IsExported() || fn.Type.Results == nil {
				continue
			}
			ptr, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			recv, ok := ptr.X.(*ast.Ident)
			if !ok || recv.Name != "AppService" {
				continue
			}
			for _, field := range fn.Type.Results.List {
				id, ok := field.Type.(*ast.Ident)
				if !ok || id.Name != "error" {
					continue
				}
				count++
				guarded := false
				if len(field.Names) == 1 && len(fn.Body.List) > 0 {
					if deferStmt, ok := fn.Body.List[0].(*ast.DeferStmt); ok {
						if sel, ok := deferStmt.Call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Present" && len(deferStmt.Call.Args) == 1 {
							if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "apperrors" {
								if addr, ok := deferStmt.Call.Args[0].(*ast.UnaryExpr); ok && addr.Op == token.AND {
									if target, ok := addr.X.(*ast.Ident); ok && target.Name == field.Names[0].Name {
										guarded = true
									}
								}
							}
						}
					}
				}
				if !guarded {
					t.Errorf("%s lacks a public error boundary", fn.Name.Name)
				}
			}
		}
	}
	if count == 0 {
		t.Fatal("no service methods checked")
	}
}
