package codegraph

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGraphBoundedPartialCoverageAndReplacement(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 110; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("bad%03d.js", i)), []byte("function {"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	m := New(t.Context(), nil, nil, nil)
	defer m.Close()
	p := Project{ID: "bounded", Root: dir}
	r, err := m.Query(t.Context(), p, Request{Operation: "status"})
	if err != nil || r.Status.Complete || r.Status.SkippedCount != 110 || len(r.Status.Skipped) != 100 {
		t.Fatalf("unbounded or hidden partial coverage: %+v %v", r.Status, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "valid.js"), []byte("function Target(){}"), 0600); err != nil {
		t.Fatal(err)
	}
	// A previously published snapshot still occupies memory during replacement.
	m.mu.Lock()
	m.projects[p.ID].status.Bytes = MaxIndexBytes - 1
	m.mu.Unlock()
	if err := m.Rescan(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	r, err = m.Query(t.Context(), p, Request{Operation: "search", Query: "Target"})
	if err != nil || len(r.Symbols) != 0 || r.Status.SkippedCount != 111 || r.Status.Complete {
		t.Fatal("replacement exceeded budget", r, err)
	}
	if err := m.Rescan(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	r, err = m.Query(t.Context(), p, Request{Operation: "search", Query: "Target"})
	if err != nil || len(r.Symbols) != 1 {
		t.Fatal("budget did not recover", r, err)
	}
}

func TestGraphCancellationAndKnownWriteSymbol(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "main.js")
	if err := os.WriteFile(file, []byte("function Target(){\n return 1;\n}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m := New(t.Context(), nil, nil, nil)
	defer m.Close()
	p := Project{ID: "writes", Root: dir}
	if symbol := m.KnownWriteSymbol(t.Context(), p, "main.js", 2, 2); symbol != "" {
		t.Fatal("write started indexing")
	}
	if err := m.Rescan(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	if symbol := m.KnownWriteSymbol(t.Context(), p, "main.js", 2, 2); symbol == "" {
		t.Fatal("known symbol missing")
	}
	if err := os.WriteFile(file, []byte("function Changed(){}"), 0600); err != nil {
		t.Fatal(err)
	}
	if symbol := m.KnownWriteSymbol(t.Context(), p, "main.js", 2, 2); symbol != "" {
		t.Fatal("stale symbol advertised")
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := parseFile(ctx, "stress.js", strings.Repeat("f(", 100000)+"0"+strings.Repeat(")", 100000), "test")
	if err == nil || time.Since(started) > 2*time.Second {
		t.Fatal("unbounded nested parse", err)
	}
}

func TestGraphLanguagesAndBoundaries(t *testing.T) {
	fixtures := map[string]string{
		"main.go":  "package demo\nfunc Target() {}\nfunc Caller() { Target() }\n",
		"plain.js": "function Target() {}\nfunction Caller() { Target(); }\n",
		"view.jsx": "function Target() { return <div/>; }\nfunction Caller() { return Target(); }\n",
		"typed.ts": "function Target(): number { return 1; }\nfunction Caller(): number { return Target(); }\n",
		"view.tsx": "function Target() { return <div/>; }\nfunction Caller() { return Target(); }\n",
		"main.py":  "def Target():\n    return 1\ndef Caller():\n    return Target()\n",
	}
	for name, source := range fixtures {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			m := New(context.Background(), nil, nil, nil)
			defer m.Close()
			p := Project{ID: "test", Root: dir}
			r, err := m.Query(context.Background(), p, Request{Operation: "callers", Symbol: "Target"})
			if err != nil {
				t.Fatal(err)
			}
			if !r.Status.Complete || len(r.Edges) != 1 || r.Edges[0].To == "" {
				t.Fatalf("missing resolved call: %+v", r)
			}
			if _, err = m.Query(context.Background(), Project{ID: "test", Root: t.TempDir()}, Request{Operation: "search"}); err == nil {
				t.Fatal("project root substitution allowed")
			}
		})
	}
}

func TestGraphImportsNestedDefinitionsAndHeritage(t *testing.T) {
	cases := []struct {
		name    string
		files   map[string]string
		symbol  string
		callers int
	}{
		{"go-alias", map[string]string{"go.mod": "module example.test/app\n", "lib/lib.go": "package lib\nfunc Target() {}\n", "main.go": "package app\nimport alias \"example.test/app/lib\"\nfunc Caller(){alias.Target()}\n"}, "Target", 1},
		{"python-alias", map[string]string{"lib.py": "def Target():\n    return 1\n", "main.py": "from lib import Target as Alias\ndef Caller():\n    return Alias()\n"}, "Target", 1},
		{"js-export-alias", map[string]string{"lib.js": "function Target(){}\nexport {Target as Published};\n", "main.js": "import {Published as Alias} from './lib';\nfunction Caller(){Alias();}\n"}, "Target", 1},
		{"js-private", map[string]string{"lib.js": "function Target(){}\n", "main.js": "import {Target} from './lib';\nfunction Caller(){Target();}\n"}, "Target", 0},
		{"python-nested", map[string]string{"main.py": "def Outer():\n    def Target():\n        return 1\n    return Target()\ndef Other():\n    return Target()\n"}, "Target", 1},
		{"python-inheritance", map[string]string{"main.py": "class Parent:\n    pass\nclass Child(Parent):\n    pass\n"}, "Parent", 1},
		{"ts-inheritance", map[string]string{"main.ts": "class Parent {}\nclass Child extends Parent {}\n"}, "Parent", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, source := range c.files {
				p := filepath.Join(dir, name)
				if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(source), 0600); err != nil {
					t.Fatal(err)
				}
			}
			m := New(t.Context(), nil, nil, nil)
			defer m.Close()
			r, err := m.Query(t.Context(), Project{ID: "test", Root: dir}, Request{Operation: "callers", Symbol: c.symbol})
			if err != nil || !r.Status.Complete || len(r.Edges) != c.callers {
				t.Fatalf("%+v %v", r, err)
			}
		})
	}
}

func TestGraphFingerprintRevocationShutdownAndRestart(t *testing.T) {
	dir := t.TempDir()
	p := Project{ID: "test", Root: dir}
	write := func(source string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("package demo\nfunc Before(){}\n")
	var allowed atomic.Bool
	allowed.Store(true)
	var changes sync.Map
	changed := func(name string) any { v, _ := changes.Load(name); return v }
	m := New(t.Context(), nil, func(Project) bool { return allowed.Load() }, func(_ context.Context, _ Project, name, hash string) { changes.Store(name, hash) })
	if err := m.Rescan(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	first := changed("main.go")
	write("package demo\nfunc After(){}\n")
	if err := m.Rescan(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	if changed("main.go") == first || changed("main.go") == "" {
		t.Fatal("missing change notification")
	}
	if err := os.Remove(filepath.Join(dir, "main.go")); err != nil {
		t.Fatal(err)
	}
	if err := m.Rescan(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	if changed("main.go") != "missing" {
		t.Fatal(changed("main.go"))
	}
	for _, name := range []string{"../secret.go", "a/../main.go", "C:/secret.go", "/etc/passwd"} {
		if _, err := Fingerprint(p, name); err == nil {
			t.Fatal("fingerprint escaped", name)
		}
	}
	allowed.Store(false)
	if _, err := m.Query(t.Context(), p, Request{Operation: "status"}); err == nil {
		t.Fatal("revoked project survived")
	}
	m.Close()
	if _, err := m.Query(t.Context(), p, Request{Operation: "status"}); err == nil {
		t.Fatal("closed manager revived")
	}
	next := New(t.Context(), nil, nil, nil)
	defer next.Close()
	r, err := next.Query(t.Context(), p, Request{Operation: "status"})
	if err != nil || r.Status.Generation != 1 || r.Status.Indexed != 0 {
		t.Fatal(r, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := parseFile(ctx, "main.go", "package demo\nfunc Main(){}\n", ""); err == nil {
		t.Fatal("cancelled parser returned a file")
	}
}

func TestGraphImportsScopesChangesAndIncompleteFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(name, source string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("a.ts", "export function Ziel() { return 1; }\n")
	write("b.ts", "import { Ziel as Alias } from './a';\nfunction Caller() { return Alias(); }\nfunction Dynamic(Ziel: any) { return Ziel(); }\n")
	write("bad.py", "def broken(:\n")
	write("large.py", strings.Repeat(" ", MaxFileBytes+1))
	m := New(context.Background(), nil, nil, nil)
	defer m.Close()
	p := Project{ID: "test", Root: dir}
	r, err := m.Query(context.Background(), p, Request{Operation: "callers", Symbol: "Ziel"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status.Complete || len(r.Status.Skipped) != 2 || len(r.Edges) != 1 {
		t.Fatalf("coverage/import resolution: %+v", r)
	}
	r, err = m.Query(context.Background(), p, Request{Operation: "callees", Symbol: "Dynamic"})
	if err != nil || len(r.Edges) != 1 || r.Edges[0].To != "" {
		t.Fatal("parameter falsely resolved", r, err)
	}
	write("a.ts", "export function Changed() { return 2; }\n")
	if err = os.Remove(filepath.Join(dir, "b.ts")); err != nil {
		t.Fatal(err)
	}
	r, err = m.Query(context.Background(), p, Request{Operation: "rescan"})
	if err != nil {
		t.Fatal(err)
	}
	r, err = m.Query(context.Background(), p, Request{Operation: "search", Query: "Ziel"})
	if err != nil || len(r.Symbols) != 0 {
		t.Fatal("stale symbols", r, err)
	}
	r, err = m.Query(context.Background(), p, Request{Operation: "search", Query: "Changed"})
	if err != nil || len(r.Symbols) != 1 {
		t.Fatal("missing change", r, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = m.Rescan(ctx, p); err == nil {
		t.Fatal("cancelled scan published")
	}
}

func TestGraphRevocationAndUnicode(t *testing.T) {
	f, err := parseFile(context.Background(), "a.py", "def grüße():\n    return 1\n", "x")
	if err != nil || len(f.symbols) != 1 || f.symbols[0].Name != "grüße" {
		t.Fatal("unicode", f, err)
	}
	dir := t.TempDir()
	m := New(context.Background(), func() bool { return false }, nil, nil)
	defer m.Close()
	if _, err = m.Query(context.Background(), Project{ID: "test", Root: dir}, Request{Operation: "status"}); err == nil {
		t.Fatal("disabled graph started")
	}
}
