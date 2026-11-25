package chibisensei

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/security"
)

func TestRunFilesLayout(t *testing.T) {
	dir := t.TempDir()
	id := "11111111-2222-4333-8444-555555555555"
	for rel, n := range map[string]int{
		"runs/" + id + "/out/a.txt": 3,
		"models/" + id + "/m.bin":   5,
		"base/b.gguf":               7,
	} {
		if err := writeChibiFile(dir, rel, bytes.Repeat([]byte("x"), n)); err != nil {
			t.Fatal(err)
		}
	}
	if got := StorageBytes(dir); got != 15 {
		t.Fatalf("StorageBytes = %d, want 15", got)
	}
	if err := RemoveRunFiles(dir, id); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{RunDir(dir, id), ModelDir(dir, id)} {
		if _, err := os.Stat(p); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("%s still exists: %v", p, err)
		}
	}
	if _, err := os.Stat(filepath.Join(BaseDir(dir), "b.gguf")); err != nil {
		t.Fatalf("base file removed: %v", err)
	}
	if err := RemoveRunFiles(dir, id); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if err := RemoveRunFiles(dir, "x"); err == nil {
		t.Fatal("invalid id accepted")
	}

	token := "tok-" + strings.Repeat("q7", 16)
	security.RegisterSensitive(token)
	got := string(logTailText(fakeKernelLogJSON("line one", "secret "+token)))
	if !strings.Contains(got, "line one") || strings.Contains(got, token) {
		t.Fatalf("log tail not scrubbed: %q", got)
	}
	body := append(bytes.Repeat([]byte("a"), 100<<10-1), 'Z')
	tail := logTailText(body)
	if len(tail) != 65536 || tail[len(tail)-1] != 'Z' {
		t.Fatalf("plain tail: %d bytes", len(tail))
	}
	if b, err := ReadLogTail(dir, "99999999-2222-4333-8444-555555555555"); b != nil || err != nil {
		t.Fatalf("unknown run: %v %v", b, err)
	}
	if err := writeLogTail(dir, id, body); err != nil {
		t.Fatal(err)
	}
	if b, err := ReadLogTail(dir, id); err != nil || len(b) != 65536 {
		t.Fatalf("stored tail: %d %v", len(b), err)
	}
}

func TestLogTailTextEventStream(t *testing.T) {
	entries := []streamEntry{{"stdout", 1.5, "first line\n"}, {"stderr", 2.5, "second line\n"}}
	got := string(logTailText(eventStreamBody(entries)))
	if got != "first line\nsecond line\n" {
		t.Fatalf("tail of an event stream: %q", got)
	}
	if strings.Contains(got, "data:") || strings.Contains(got, "stream_name") {
		t.Fatalf("raw event lines in the tail: %q", got)
	}
}
