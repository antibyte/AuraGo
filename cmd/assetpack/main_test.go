package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/webassets"
)

func TestDeterministicProductionPack(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, dir := range []string{"assets", "ui/testdata", "ui/js"} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"assets/web-assets.json": `{"version":1,"roots":[{"source":"ui","target":"ui","directories":["js"],"extensions":[".html",".js"]}]}`,
		"ui/index.html":          "hello\r\nworld\r\n", "ui/js/app.js": "local();\n", "ui/js/app.test.js": "test only", "ui/testdata/secret.html": "not production", "ui/js/LICENSE": "license text", "ui/AGENTS.md": "not production",
	}
	for file, b := range files {
		if err := os.WriteFile(file, []byte(b), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := build("out", "installed", "v-test"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile("out/web-assets.json")
	var first webassets.Pin
	json.Unmarshal(b, &first)
	s := webassets.Open("installed", first)
	if !s.Ready() {
		t.Fatal(s.Error)
	}
	for _, excluded := range []string{"ui/js/app.test.js", "ui/testdata/secret.html", "ui/AGENTS.md"} {
		if _, err := s.Open(excluded); err == nil {
			t.Fatalf("pack included %s", excluded)
		}
	}
	s.Close()
	os.WriteFile("ui/index.html", []byte("hello\nworld\n"), 0644)
	if err := build("out", "installed", "v-test"); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join("out", "web-assets.json"))
	if string(after) != string(b) {
		t.Fatal("line endings or repeat build changed resource identity")
	}
}
