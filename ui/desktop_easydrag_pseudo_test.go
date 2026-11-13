package ui

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The pseudo-locale is a development switch, never a language: no language list may contain it.
func TestEasyDragPseudoLocaleIsInNoLanguageList(t *testing.T) {
	real := map[string]bool{}
	for lang := range easyDragPluralForms {
		real[lang] = true
	}
	// 1. every locale file under ui/lang is a real language (meta.json is the one other file)
	seen := 0
	err := filepath.WalkDir("lang", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".json") {
			return err
		}
		seen++
		name := strings.TrimSuffix(filepath.Base(path), ".json")
		if name != "meta" && !real[name] {
			t.Errorf("%s is not one of the 16 languages", path)
		}
		return nil
	})
	if err != nil || seen < 400 {
		t.Fatalf("walked %d locale files (%v); is the scan broken?", seen, err)
	}
	// 2. the language switcher of the shared UI lists real codes only
	shared := readDesktopAssetText(t, "js/shared/shared-core.js")
	codes := regexp.MustCompile(`\{ code: '([^']+)', label:`).FindAllStringSubmatch(shared, -1)
	if len(codes) != 16 {
		t.Fatalf("language switcher lists %d codes, want 16", len(codes))
	}
	for _, m := range codes {
		if !real[m[1]] {
			t.Errorf("the language switcher lists %q", m[1])
		}
	}
	// 3. the switch name lives in easydrag-format.js only
	for _, name := range easyDragModules {
		if name != "easydrag-format.js" && strings.Contains(readDesktopAssetText(t, "js/desktop/apps/"+name), "aurago.easydrag.pseudo") {
			t.Errorf("%s names the pseudo switch; only easydrag-format.js may", name)
		}
	}
	// 4. EasyDrag takes its t through the wrapper, in one place
	if !strings.Contains(readDesktopAssetText(t, "js/desktop/apps/easydrag.js"), "t: ED.format.wrapT(ctx.t)") {
		t.Error("easydrag.js does not wrap ctx.t with ED.format.wrapT")
	}
}
