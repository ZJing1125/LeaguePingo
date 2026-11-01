package hold

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// This runs on portable CI too: native callbacks must not acquire application
// locks, query processes or manage timers before forwarding the original input.
func TestNativeCallbacksContainNoBlockingWork(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "monitor_windows.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	callbacks := 0
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "NewCallback" {
			return true
		}
		callbacks++
		ast.Inspect(call, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.Ident:
				switch v.Name {
				case "GameForeground", "configuredKeyDown", "setTimer", "killTimer", "keyState", "OpenProcess", "QueryFullProcessImageName":
					t.Errorf("native callback references blocking operation %s", v.Name)
				}
			case *ast.SelectorExpr:
				switch v.Sel.Name {
				case "Call", "NewProc", "Lock", "Input", "Desired", "CurrentKey", "Pressed", "Release", "NewTimer", "InvokeSync":
					t.Errorf("native callback calls %s", v.Sel.Name)
				}
			}
			return true
		})
		return false
	})
	// Follow the callback-safe State helpers too: moving a mutex/native call
	// one function deeper must not make this contract appear satisfied.
	state, err := parser.ParseFile(token.NewFileSet(), "state.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, declaration := range state.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok || (fn.Name.Name != "capture" && fn.Name.Name != "interrupt" && fn.Name.Name != "signal") {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					switch sel.Sel.Name {
					case "Load", "CompareAndSwap", "signal":
					default:
						t.Errorf("callback helper %s calls %s", fn.Name.Name, sel.Sel.Name)
					}
				}
			}
			return true
		})
	}

	if callbacks != 2 {
		t.Fatalf("expected keyboard and foreground callbacks, got %d", callbacks)
	}
}
