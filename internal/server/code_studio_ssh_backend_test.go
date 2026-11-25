package server

import (
	"context"
	"errors"
	"strings"
	"testing"
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
