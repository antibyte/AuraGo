package tools

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/config"
)

// The Landlock sandbox gets its Docker endpoint from the ExtraEnv slice built
// in cmd/aurago/main.go. The unsandboxed shell filter must hide every name that
// slice injects, under the same gate: main.go injects only inside
// `if cfg.Docker.Enabled`, and RuntimePermissionsFromConfig copies that flag
// into the DockerEnabled field requireDockerPermission reads.
func TestDockerClientEnvNamesCoverTheLandlockExtraEnvVariable(t *testing.T) {
	mainPath := filepath.Join("..", "..", "cmd", "aurago", "main.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, mainPath, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", mainPath, err)
	}
	scan := scanLandlockExtraEnv(fset, file)
	for _, problem := range scan.problems {
		t.Error(problem)
	}
	if scan.extraEnvFields != 1 {
		t.Fatalf("expected exactly one ExtraEnv field in %s, found %d", mainPath, scan.extraEnvFields)
	}
	for _, name := range scan.injected {
		if !dockerClientEnvNames[strings.ToUpper(name)] {
			t.Errorf("main.go injects %s into the Landlock sandbox; add it to dockerClientEnvNames so unsandboxed shells hide it under the same gate", name)
		}
	}
	if strings.Join(scan.injected, ",") != "DOCKER_HOST" {
		t.Fatalf("Landlock ExtraEnv injects %v; expected only DOCKER_HOST (update dockerClientEnvNames and this test together)", scan.injected)
	}

	cfg := &config.Config{}
	for _, enabled := range []bool{false, true} {
		cfg.Docker.Enabled = enabled
		if got := RuntimePermissionsFromConfig(cfg).DockerEnabled; got != enabled {
			t.Fatalf("RuntimePermissionsFromConfig(docker.enabled=%v).DockerEnabled = %v; the shell filter gate must follow cfg.Docker.Enabled", enabled, got)
		}
	}
}

// The scanner must flag what it guards against; checked on synthetic sources
// so cmd/aurago/main.go itself is never weakened for the check.
func TestScanLandlockExtraEnvFlagsUngatedAndUnreadableInjections(t *testing.T) {
	const declare = "var extraEnv []string\n"
	cases := []struct {
		name         string
		body         string
		wantInjected string
		wantProblem  string
	}{
		{"gated", declare + `if cfg.Docker.Enabled { extraEnv = append(extraEnv, "DOCKER_HOST="+host) }`, "DOCKER_HOST", ""},
		{"other name is reported", declare + `if cfg.Docker.Enabled { extraEnv = append(extraEnv, "DOCKER_TLS_VERIFY=1") }`, "DOCKER_TLS_VERIFY", ""},
		{"ungated", declare + `extraEnv = append(extraEnv, "DOCKER_HOST="+host)`, "DOCKER_HOST", "inside `if cfg.Docker.Enabled`"},
		{"else branch", declare + `if cfg.Docker.Enabled { } else { extraEnv = append(extraEnv, "DOCKER_HOST="+host) }`, "DOCKER_HOST", "inside `if cfg.Docker.Enabled`"},
		{"other gate", declare + `if cfg.Agent.AllowShell { extraEnv = append(extraEnv, "DOCKER_HOST="+host) }`, "DOCKER_HOST", "inside `if cfg.Docker.Enabled`"},
		{"computed name", declare + `if cfg.Docker.Enabled { extraEnv = append(extraEnv, name+"="+host) }`, "", "string literal"},
		{"replaced slice", declare + `if cfg.Docker.Enabled { extraEnv = other }`, "", "only be built from"},
		{"initialized var spec", `var extraEnv = []string{"DOCKER_CERT_PATH=/certs"}`, "DOCKER_CERT_PATH", "inside `if cfg.Docker.Enabled`"},
		{"initialized from a call", `var extraEnv = buildExtraEnv()`, "", "only be built from"},
		{"short declaration", `extraEnv := []string{"DOCKER_CONTEXT=remote"}`, "DOCKER_CONTEXT", "inside `if cfg.Docker.Enabled`"},
		{"index assignment", declare + `if cfg.Docker.Enabled { extraEnv = append(extraEnv, "DOCKER_HOST="+host) }` + "\n" + `extraEnv[0] = "DOCKER_TLS_VERIFY=1"`, "DOCKER_HOST,DOCKER_TLS_VERIFY", "inside `if cfg.Docker.Enabled`"},
		{"gated index assignment", declare + `if cfg.Docker.Enabled { extraEnv = append(extraEnv, "DOCKER_HOST="+host); extraEnv[0] = "DOCKER_CONFIG=/cfg" }`, "DOCKER_HOST,DOCKER_CONFIG", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := "package p\nfunc f() {\n" + tc.body + "\nInit(Config{ExtraEnv: extraEnv})\n}\n"
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "synthetic.go", src, 0)
			if err != nil {
				t.Fatalf("parse synthetic source: %v", err)
			}
			scan := scanLandlockExtraEnv(fset, file)
			if got := strings.Join(scan.injected, ","); got != tc.wantInjected {
				t.Fatalf("injected = %q, want %q", got, tc.wantInjected)
			}
			problems := strings.Join(scan.problems, "\n")
			if tc.wantProblem == "" && problems != "" {
				t.Fatalf("unexpected problems: %s", problems)
			}
			if tc.wantProblem != "" && !strings.Contains(problems, tc.wantProblem) {
				t.Fatalf("problems = %q, want one containing %q", problems, tc.wantProblem)
			}
		})
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "synthetic.go", "package p\nfunc f() { Init(Config{ExtraEnv: []string{\"DOCKER_HOST=x\"}}) }\n", 0)
	if err != nil {
		t.Fatalf("parse synthetic source: %v", err)
	}
	if scan := scanLandlockExtraEnv(fset, file); scan.extraEnvFields != 1 || !strings.Contains(strings.Join(scan.problems, "\n"), "must be the extraEnv slice") {
		t.Fatalf("an inline ExtraEnv value must be flagged, got %+v", scan)
	}
}

type landlockExtraEnvScan struct {
	injected       []string
	extraEnvFields int
	problems       []string
}

// scanLandlockExtraEnv collects every entry put into extraEnv (append,
// composite-literal initializers in var specs or assignments, and index
// assignments), counts the ExtraEnv fields, and reports injections outside
// `if cfg.Docker.Enabled`, entries without a "NAME=" literal prefix, other ways
// of building extraEnv, and ExtraEnv values other than extraEnv.
func scanLandlockExtraEnv(fset *token.FileSet, file *ast.File) landlockExtraEnvScan {
	var scan landlockExtraEnvScan
	var stack []ast.Node
	addProblem := func(pos token.Pos, format string, args ...any) {
		scan.problems = append(scan.problems, fset.Position(pos).String()+": "+fmt.Sprintf(format, args...))
	}
	// inject records entries and checks the gate around the injecting node.
	inject := func(node ast.Node, entries []ast.Expr) {
		for _, entry := range entries {
			literal, ok := leftmostStringLiteral(entry)
			name, _, hasValue := strings.Cut(literal, "=")
			if !ok || !hasValue || name == "" {
				addProblem(node.Pos(), "extraEnv entry %s must start with a \"NAME=\" string literal", types.ExprString(entry))
				continue
			}
			scan.injected = append(scan.injected, name)
		}
		if !insideIfBody(stack, node, "cfg.Docker.Enabled") {
			addProblem(node.Pos(), "extraEnv must only be extended inside `if cfg.Docker.Enabled`")
		}
	}
	// injectValue handles a whole-slice value assigned to or initializing extraEnv.
	injectValue := func(node ast.Node, value ast.Expr) {
		switch v := value.(type) {
		case *ast.CallExpr:
			if fn, ok := v.Fun.(*ast.Ident); ok && fn.Name == "append" && len(v.Args) >= 2 {
				inject(node, v.Args[1:])
				return
			}
		case *ast.CompositeLit:
			inject(node, v.Elts)
			return
		case *ast.Ident:
			if v.Name == "nil" {
				return
			}
		}
		addProblem(node.Pos(), "extraEnv may only be built from append(extraEnv, \"NAME=\"+value) or a []string{\"NAME=\"+value} literal, got %s", types.ExprString(value))
	}
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		switch node := n.(type) {
		case *ast.KeyValueExpr:
			if key, ok := node.Key.(*ast.Ident); ok && key.Name == "ExtraEnv" {
				scan.extraEnvFields++
				if value, ok := node.Value.(*ast.Ident); !ok || value.Name != "extraEnv" {
					addProblem(node.Pos(), "ExtraEnv must be the extraEnv slice, got %s", types.ExprString(node.Value))
				}
			}
		case *ast.ValueSpec:
			for i, name := range node.Names {
				if name.Name != "extraEnv" || len(node.Values) == 0 {
					continue
				}
				if len(node.Values) != len(node.Names) {
					addProblem(node.Pos(), "extraEnv must be initialized from its own value")
					continue
				}
				injectValue(node, node.Values[i])
			}
		case *ast.AssignStmt:
			for i, lhs := range node.Lhs {
				switch target := lhs.(type) {
				case *ast.Ident:
					if target.Name != "extraEnv" {
						continue
					}
					if len(node.Rhs) != len(node.Lhs) {
						addProblem(node.Pos(), "extraEnv must be assigned its own value")
						continue
					}
					injectValue(node, node.Rhs[i])
				case *ast.IndexExpr:
					if base, ok := target.X.(*ast.Ident); !ok || base.Name != "extraEnv" {
						continue
					}
					if node.Tok != token.ASSIGN || len(node.Rhs) != len(node.Lhs) {
						addProblem(node.Pos(), "extraEnv index assignments must assign a \"NAME=\" string literal")
						continue
					}
					inject(node, []ast.Expr{node.Rhs[i]})
				}
			}
		}
		return true
	})
	return scan
}

// leftmostStringLiteral returns the unquoted string literal that starts expr,
// following "literal" + value concatenations.
func leftmostStringLiteral(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return "", false
		}
		value, err := strconv.Unquote(e.Value)
		return value, err == nil
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return "", false
		}
		return leftmostStringLiteral(e.X)
	case *ast.ParenExpr:
		return leftmostStringLiteral(e.X)
	}
	return "", false
}

// insideIfBody reports whether node sits in the then-branch of an enclosing
// if statement whose condition renders as cond.
func insideIfBody(stack []ast.Node, node ast.Node, cond string) bool {
	for _, ancestor := range stack {
		ifStmt, ok := ancestor.(*ast.IfStmt)
		if !ok || types.ExprString(ifStmt.Cond) != cond {
			continue
		}
		if node.Pos() >= ifStmt.Body.Pos() && node.End() <= ifStmt.Body.End() {
			return true
		}
	}
	return false
}
