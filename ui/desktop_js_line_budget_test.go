package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget(t *testing.T) {
	t.Parallel()

	root := filepath.Join("js", "desktop")
	knownOversizedContinuations := map[string]bool{
		filepath.ToSlash(filepath.Join("js", "desktop", "apps", "bookshelf.js")):                   true,
		filepath.ToSlash(filepath.Join("js", "desktop", "apps", "galaxa-deluxe.js")):               true,
		filepath.ToSlash(filepath.Join("js", "desktop", "apps", "game-maker-studio.js")):           true,
		filepath.ToSlash(filepath.Join("js", "desktop", "core", "desktop-foundation.js")):          true,
		filepath.ToSlash(filepath.Join("js", "desktop", "core", "menus-and-routing.js")):           true,
		filepath.ToSlash(filepath.Join("js", "desktop", "core", "window-shell-runtime.js")):        true,
		filepath.ToSlash(filepath.Join("js", "desktop", "apps", "agent-chat.js")):                  true,
		filepath.ToSlash(filepath.Join("js", "desktop", "apps", "quickconnect-launchpad-chat.js")): true,
		filepath.ToSlash(filepath.Join("js", "desktop", "apps", "galaxa-sprites.js")):              true,
		filepath.ToSlash(filepath.Join("js", "desktop", "apps", "sheets.js")):                      true,
		filepath.ToSlash(filepath.Join("js", "desktop", "apps", "code-studio", "core.js")):         true,
		filepath.ToSlash(filepath.Join("js", "desktop", "apps", "openscad.js")):                    true,
		// Pixel Pro: pure image libraries (they must stay DOM-free single files because the worker imports them) and
		// the large feature modules of the editor.
		filepath.ToSlash(filepath.Join("js", "desktop", "apps", "pixel-core.js")):       true,
		filepath.ToSlash(filepath.Join("js", "desktop", "apps", "pixel-cutout.js")):     true,
		filepath.ToSlash(filepath.Join("js", "desktop", "apps", "pixel-filters.js")):    true,
		filepath.ToSlash(filepath.Join("js", "desktop", "apps", "pixel-layers.js")):     true,
		filepath.ToSlash(filepath.Join("js", "desktop", "apps", "pixel-lib-cutout.js")): true,
		filepath.ToSlash(filepath.Join("js", "desktop", "apps", "pixel-lib-fx.js")):     true,
		filepath.ToSlash(filepath.Join("js", "desktop", "apps", "pixel-lib-gen.js")):    true,
		filepath.ToSlash(filepath.Join("js", "desktop", "apps", "pixel-selection.js")):  true,
	}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == "vendor" || entry.Name() == "bundles" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".js" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lines := strings.Count(string(data), "\n")
		if len(data) > 0 && data[len(data)-1] != '\n' {
			lines++
		}
		if lines >= 1100 && !knownOversizedContinuations[filepath.ToSlash(path)] {
			t.Errorf("%s has %d lines, want < 1100", filepath.ToSlash(path), lines)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestVirtualDesktopJSUsesSemanticChunkNames(t *testing.T) {
	t.Parallel()

	root := filepath.Join("js", "desktop")
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == "vendor" || entry.Name() == "bundles" {
				return filepath.SkipDir
			}
			return nil
		}
		name := entry.Name()
		if strings.Contains(name, "-part-") || strings.HasPrefix(name, "part-") {
			t.Errorf("%s uses a mechanical chunk name", filepath.ToSlash(path))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
