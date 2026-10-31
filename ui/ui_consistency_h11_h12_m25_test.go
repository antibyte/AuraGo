package ui

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUIConsistencyNoUnstyledCfgButtonClasses(t *testing.T) {
	for _, root := range []string{"js", "cfg"} {
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".js") {
				return nil
			}
			b, _ := os.ReadFile(p)
			for _, c := range []string{"cfg-btn-primary", "cfg-btn-sm", "cfg-btn-row", `class="cfg-btn"`, `"cfg-btn `} {
				if strings.Contains(string(b), c) {
					t.Errorf("%s uses unstyled class %q", p, c)
				}
			}
			return nil
		})
	}
}

func TestUIConsistencySetupUsesSharedToast(t *testing.T) {
	b, err := os.ReadFile("js/setup/main.js")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "function showToast") || strings.Contains(string(b), "toastContainer") {
		t.Error("setup must use the shared showToast, not a local innerHTML variant")
	}
}

func TestUIConsistencyDashboardConfirmFailsClosed(t *testing.T) {
	_ = filepath.WalkDir("js/dashboard", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".js") {
			return nil
		}
		b, _ := os.ReadFile(p)
		if strings.Contains(string(b), "typeof showConfirm") {
			t.Errorf("%s has a typeof showConfirm fallback", p)
		}
		return nil
	})
}
