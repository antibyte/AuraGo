package cydcreator

import (
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestPackageLayering keeps the domain package from directly importing server,
// agent, tool and Docker code; those layers reach it through interfaces. Only
// direct imports are checked: indirect imports (for example via internal/config)
// are expected in later plans.
func TestPackageLayering(t *testing.T) {
	forbidden := []string{
		"aurago/internal/server", "aurago/internal/agent", "aurago/internal/tools",
		"aurago/internal/dockerutil", "github.com/docker",
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	checked := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		checked++
		for _, imp := range f.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			for _, bad := range forbidden {
				if path == bad || strings.HasPrefix(path, bad+"/") {
					t.Errorf("%s imports %s; cydcreator must not depend on server, agent, tools or Docker code", name, path)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no package files found")
	}
}
