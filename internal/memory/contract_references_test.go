package memory

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func TestMemoryContractTestReferencesResolve(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate memory contract")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	contract, err := os.ReadFile(filepath.Join(filepath.Dir(file), "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	defined := make(map[string]bool)
	for _, base := range []string{"internal", "ui", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, base), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, "_test.go") {
				return nil
			}
			tree, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			for _, decl := range tree.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if ok && fn.Recv == nil && strings.HasPrefix(fn.Name.Name, "Test") {
					defined[fn.Name.Name] = true
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	references := regexp.MustCompile("`(Test[A-Za-z0-9_]+\\*?)`").FindAllStringSubmatch(string(contract), -1)
	if len(references) == 0 {
		t.Fatal("memory contract contains no executable verification references")
	}
	for _, match := range references {
		reference := match[1]
		resolved := defined[reference]
		if strings.HasSuffix(reference, "*") {
			prefix := strings.TrimSuffix(reference, "*")
			for name := range defined {
				if strings.HasPrefix(name, prefix) {
					resolved = true
					break
				}
			}
		}
		if !resolved {
			t.Errorf("memory contract test reference %q has no Go test definition", reference)
		}
	}
}
