package server

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestRuntimeCollectsActionPerformersThroughOneFactory(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "server.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	factories := 0
	ast.Inspect(file, func(node ast.Node) bool {
		if literal, ok := node.(*ast.CompositeLit); ok {
			name := ""
			switch value := literal.Type.(type) {
			case *ast.Ident:
				name = value.Name
			case *ast.SelectorExpr:
				name = value.Sel.Name
			}
			if strings.HasSuffix(name, "ActionPerformer") {
				t.Error("construct action performers inside GetActionPerformers, not the runtime")
			}
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if function, ok := call.Fun.(*ast.SelectorExpr); ok && function.Sel.Name == "GetActionPerformers" {
			if owner, ok := function.X.(*ast.Ident); ok && owner.Name == "action_provider" {
				factories++
			}
		}
		if function, ok := call.Fun.(*ast.SelectorExpr); ok && (function.Sel.Name == "Performers" ||
			strings.HasPrefix(function.Sel.Name, "New") && strings.HasSuffix(function.Sel.Name, "Performer")) {
			t.Error("construct action performers inside GetActionPerformers, not the runtime")
		}
		if function, ok := call.Fun.(*ast.Ident); ok && function.Name == "append" && len(call.Args) > 0 {
			if list, ok := call.Args[0].(*ast.Ident); ok && list.Name == "actionPerformers" {
				t.Error("runtime action performers must be collected inside GetActionPerformers")
			}
		}
		return true
	})
	if factories != 1 {
		t.Fatalf("runtime has %d action performer factories, want one", factories)
	}
}
