package retronet

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// allowedNonStdImports lists the only imports outside the standard library
// that production files of this package may use (overview, Contract A).
var allowedNonStdImports = map[string]bool{
	"aurago/internal/security": true,
	"golang.org/x/crypto/ssh":  true,
}

func TestProductionImportsStayInsideContract(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	fset := token.NewFileSet()
	checked := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		checked++
		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatalf("%s: bad import %s", name, spec.Path.Value)
			}
			first, _, _ := strings.Cut(path, "/")
			standardLibrary := first != "aurago" && !strings.Contains(first, ".")
			if !standardLibrary && !allowedNonStdImports[path] {
				t.Errorf("%s imports %q; retronet may only import the standard library, aurago/internal/security and golang.org/x/crypto/ssh", name, path)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no production files found")
	}
}
