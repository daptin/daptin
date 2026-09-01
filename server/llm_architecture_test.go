package server

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestLLMGatewayHasOneDaptinCompositionPath(t *testing.T) {
	for _, path := range []string{"llmgateway", "metering", "schemamigration", "resource/migration", "jsonx"} {
		if _, err := os.Stat(path); err == nil {
			t.Errorf("rejected parallel package server/%s exists", path)
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}

	forbiddenIdentifiers := map[string]bool{
		"GoAIProvider": true, "ResolveLLMProviderByModel": true,
		"daptinReference": true, "gatewayID": true, "stableID": true, "optionalID": true,
		"referenceBytes": true, "referenceString": true,
		"resolveResourceReference": true, "resolveOptionalResourceReference": true,
		"loadProviders": true, "loadModels": true, "loadDeployments": true, "loadReferenceIndex": true,
	}
	standaloneConstructors := 0
	hostConstructors := 0
	forbiddenMeteringWrites := map[string]bool{"Exec": true, "ExecContext": true, "NamedExec": true, "MustExec": true}
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != "." && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		imports := make(map[string]string, len(file.Imports))
		for _, imported := range file.Imports {
			importPath, unquoteErr := strconv.Unquote(imported.Path.Value)
			if unquoteErr != nil {
				return unquoteErr
			}
			name := filepath.Base(importPath)
			if imported.Name != nil {
				name = imported.Name.Name
			}
			imports[name] = importPath
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.Ident:
				if forbiddenIdentifiers[typed.Name] {
					t.Errorf("%s contains removed LLM symbol %s", path, typed.Name)
				}
			case *ast.BasicLit:
				if typed.Kind == token.STRING {
					value, unquoteErr := strconv.Unquote(typed.Value)
					if unquoteErr == nil && value == "llm_usage" {
						t.Errorf("%s restores the removed duplicate llm_usage ledger", path)
					}
				}
			case *ast.CallExpr:
				selector, ok := typed.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if filepath.ToSlash(path) == "resource/metering.go" && forbiddenMeteringWrites[selector.Sel.Name] {
					t.Errorf("%s bypasses canonical resource writes with %s", path, selector.Sel.Name)
				}
				if selector.Sel.Name != "New" && selector.Sel.Name != "NewGateway" {
					return true
				}
				packageName, ok := selector.X.(*ast.Ident)
				if !ok {
					return true
				}
				switch imports[packageName.Name] {
				case "github.com/daptin/llmgateway":
					standaloneConstructors++
					if filepath.ToSlash(path) != "llm/gateway.go" {
						t.Errorf("%s constructs the standalone engine outside server/llm", path)
					}
				case "github.com/daptin/daptin/server/llm":
					if selector.Sel.Name == "NewGateway" {
						hostConstructors++
						if filepath.ToSlash(path) != "server.go" {
							t.Errorf("%s constructs a second Daptin LLM gateway", path)
						}
					}
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if standaloneConstructors != 1 {
		t.Fatalf("standalone engine constructors = %d, want exactly 1", standaloneConstructors)
	}
	if hostConstructors != 1 {
		t.Fatalf("Daptin LLM gateway constructors = %d, want exactly 1", hostConstructors)
	}
}
