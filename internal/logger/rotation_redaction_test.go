package logger

import (
	"aurago/internal/security"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestRotatingLogBoundsDiskAndOversizedRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.log")
	w, err := newRotatingLog(path, true, 128, 2)
	if err != nil {
		t.Fatal(err)
	}
	for range 10 {
		if _, err = w.Write([]byte(strings.Repeat("x", 70) + "\n")); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = w.Write([]byte(strings.Repeat("x", 512))); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	files, err := filepath.Glob(path + "*")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("files=%v", files)
	}
	for _, file := range files {
		stat, err := os.Stat(file)
		if err != nil || stat.Size() > 128 {
			t.Fatalf("unbounded log %s", file)
		}
	}
}
func TestPromptLogScrubsEscapedNestedSecretsAndSerializesAppends(t *testing.T) {
	secret := "fixture-secret-quote-\"\\\nline"
	security.RegisterSensitive(secret)
	dir := t.TempDir()
	entry := map[string]any{"messages": []any{map[string]any{"content": secret, "args": []string{secret}, "metadata": map[string]string{secret: "fixture"}}}}
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := AppendPromptLogEntry(dir, entry); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	data, err := os.ReadFile(PromptLogPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if !json.Valid([]byte(line)) {
			t.Fatalf("corrupt JSON: %s", line)
		}
		var value any
		_ = json.Unmarshal([]byte(line), &value)
		if strings.Contains(string(line), "fixture-secret") {
			t.Fatal("secret remained")
		}
	}
	if entry["messages"].([]any)[0].(map[string]any)["content"] != secret {
		t.Fatal("entry was mutated")
	}
}
