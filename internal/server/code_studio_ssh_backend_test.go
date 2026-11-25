package server

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCodeStudioSSHBackendFiles(t *testing.T) {
	f := startCodeStudioSSHFixture(t)
	b, _ := newCodeStudioTestSSHBackend(t, f, "/home/dev/proj")
	ctx := context.Background()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(b.Write(ctx, "/workspace/src/main.go", []byte("package main\n"), false))
	if err := b.Write(ctx, "/workspace/src/main.go", []byte("x"), true); !errors.Is(err, errCodeStudioExists) {
		t.Fatalf("createOnly: %v", err)
	}
	must(b.Write(ctx, "/workspace/src/main.go", []byte("package main // v2\n"), false))
	must(b.Write(ctx, "/workspace/README.md", []byte("# hi\n"), true))
	entries, err := b.List(ctx, "/workspace")
	must(err)
	if len(entries) != 2 || entries[0].Path != "/workspace/src" || entries[0].Type != "directory" || entries[1].Name != "README.md" {
		t.Fatalf("%+v", entries)
	}
	src, _ := b.List(ctx, "/workspace/src")
	for _, e := range src {
		if strings.HasPrefix(e.Name, ".aurago-save-") {
			t.Fatal("temp file left")
		}
	}
	probe, err := b.Probe(ctx, "/workspace/src/main.go")
	must(err)
	if probe.Path != "/workspace/src/main.go" || probe.Size != int64(len("package main // v2\n")) {
		t.Fatalf("%+v", probe)
	}
	data, err := b.Read(ctx, probe.Path, 1<<20)
	must(err)
	if string(data) != "package main // v2\n" {
		t.Fatalf("%q", data)
	}
	var se *codeStudioStatusError
	if _, err := b.Read(ctx, probe.Path, 3); !errors.As(err, &se) || se.Status != 413 {
		t.Fatalf("limit: %v", err)
	}
	var rej codeStudioProbeRejection
	if _, err := b.Probe(ctx, "/workspace/src"); !errors.As(err, &rej) {
		t.Fatalf("dir probe: %v", err)
	}
	must(b.Mkdir(ctx, "/workspace/empty/nested"))
	must(b.Move(ctx, "/workspace/src/main.go", "/workspace/cmd/app/main.go"))
	if _, err := b.Probe(ctx, "/workspace/cmd/app/main.go"); err != nil {
		t.Fatal(err)
	}
	must(b.Delete(ctx, "/workspace/cmd"))
	if err := b.Delete(ctx, "/workspace"); err == nil {
		t.Fatal("root delete allowed")
	}
	entries, _ = b.List(ctx, "/workspace")
	for _, e := range entries {
		if e.Name == "cmd" {
			t.Fatal("recursive delete left cmd")
		}
	}
}

func TestCodeStudioSSHBackendExecGitSearch(t *testing.T) {
	f := startCodeStudioSSHFixture(t)
	f.exec = func(cmd string, stdin []byte) (string, uint32) {
		s := unwrapShC(cmd)
		switch {
		case strings.Contains(s, "uname"):
			return "os=Linux\narch=aarch64\nshell=/bin/zsh\nhas=git\nhas=grep\nhas=clangd\n", 0
		case strings.Contains(s, "'grep'"):
			return "/home/dev/proj/src/main.go\x0012:func main() {\n/home/dev/proj/README.md\x003:main\n", 0
		case strings.Contains(s, "'gofmt'"):
			return strings.ToUpper(string(stdin)), 3
		}
		return "ok\n", 0
	}
	b, _ := newCodeStudioTestSSHBackend(t, f, "/home/dev/proj")
	ctx := context.Background()
	res, err := b.Exec(ctx, []string{"gofmt"}, "", []byte("package  main\n"), 5*time.Second)
	if err != nil || res.ExitCode != 3 || res.Output != "PACKAGE  MAIN\n" {
		t.Fatalf("%+v %v", res, err)
	}
	if got := unwrapShC(f.lastExec()); got != "cd '/home/dev/proj' && exec 'gofmt'" {
		t.Fatalf("%q", got)
	}
	if _, err := b.Exec(ctx, []string{"sh", "-c", "make"}, "/workspace/src", nil, time.Second); err != nil {
		t.Fatal(err)
	}
	if got := unwrapShC(f.lastExec()); !strings.HasPrefix(got, "cd '/home/dev/proj/src' && exec 'sh' '-c' 'make'") {
		t.Fatalf("%q", got)
	}
	if _, err := b.Git(ctx, time.Second, "status", "--porcelain=v1", "-z"); err != nil {
		t.Fatal(err)
	}
	if got := unwrapShC(f.lastExec()); !strings.Contains(got, "'git' '-C' '/home/dev/proj' '--literal-pathspecs' 'status'") || !strings.Contains(got, "'LC_ALL=C'") {
		t.Fatalf("%q", got)
	}
	results, truncated, err := b.Search(ctx, codeStudioSearchOptions{Query: "main", Path: "/workspace"})
	if err != nil || truncated || len(results) != 2 || results[0].Path != "/workspace/src/main.go" || results[0].Line != 12 || results[1].Path != "/workspace/README.md" {
		t.Fatalf("%+v %v %v", results, truncated, err)
	}
	if got := unwrapShC(f.lastExec()); !strings.Contains(got, "'--null'") || !strings.Contains(got, "'--' '/home/dev/proj'") {
		t.Fatalf("%q", got)
	}
	caps, err := b.probeCapabilities(ctx)
	if err != nil || caps.Arch != "aarch64" || !caps.Git || !caps.LSP.CPP || caps.LSP.Go {
		t.Fatalf("%+v %v", caps, err)
	}
}
