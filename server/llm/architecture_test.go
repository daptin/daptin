package llm

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestLLMHostUsesCanonicalResourceWrites(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	forbiddenCalls := map[string]bool{
		"Exec": true, "ExecContext": true, "NamedExec": true, "MustExec": true,
		"CreateWithTransaction": true, "UpdateWithTransaction": true, "DeleteWithTransaction": true,
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" {
			continue
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, 0)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		for _, imported := range file.Imports {
			path, unquoteErr := strconv.Unquote(imported.Path.Value)
			if unquoteErr != nil {
				t.Fatal(unquoteErr)
			}
			if path == "github.com/daptin/daptin/server/statementbuilder" && entry.Name() != "catalog_test.go" {
				t.Errorf("%s imports the SQL statement builder", entry.Name())
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if ok && forbiddenCalls[selector.Sel.Name] {
				t.Errorf("%s calls non-canonical write method %s", entry.Name(), selector.Sel.Name)
			}
			if ok {
				identifier, isIdentifier := selector.X.(*ast.Ident)
				if isIdentifier && identifier.Name == "statementbuilder" && selector.Sel.Name != "InitialiseStatementBuilder" {
					t.Errorf("%s builds SQL through statementbuilder.%s", entry.Name(), selector.Sel.Name)
				}
			}
			return true
		})
	}
}
