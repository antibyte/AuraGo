package ui

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Cookie-banner blockers ("I don't care about cookies", Fanboy/EasyList cookie lists) hide elements whose
// class is a cookieconsent name such as cc-banner or cc-modal-backdrop on every site. AuraGo therefore
// never uses the cc- prefix for classes or IDs, so a blocker cannot hide its dialogs and notices.
func TestUIAvoidsCookieConsentClassPrefix(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == filepath.Join("js", "vendor") || d.Name() == "testdata" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".css", ".js", ".mjs", ".html":
		default:
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(data)
		for i := strings.Index(text, "cc-"); i >= 0; {
			next := i + 3
			if next < len(text) && text[next] >= 'a' && text[next] <= 'z' && (i == 0 || !isClassNameByte(text[i-1])) {
				end := next
				for end < len(text) && (isClassNameByte(text[end])) {
					end++
				}
				t.Errorf("%s: cookie-blocker prone class/ID %q; use the component's own prefix (cydc-, cfgdev-, …)", path, text[i:end])
			}
			j := strings.Index(text[next:], "cc-")
			if j < 0 {
				break
			}
			i = next + j
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func isClassNameByte(b byte) bool {
	return b == '-' || b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}
