package ui

import (
	"regexp"
	"strings"
	"testing"
)

var easyDragVersionKeys = []string{
	"revision_live", "revision_ahead", "run_view_banner", "run_view_pruned", "child_history_reference",
	"pack_block_version", "approval_revision", "approval_blocked_stale", "approval_stale_help",
	"cred_purge_live_block_line", "cred_purge_live_block_advice", "publish_changes", "publish_tested_revision",
}

// No German or English value says "Revision" (any case) or "Version <number>": a published state is vN.
func TestEasyDragVersionsReadAsVN(t *testing.T) {
	placeholder := regexp.MustCompile(`\{[^}]*\}`)
	numbered := regexp.MustCompile(`(?i)\bversions?\s+\{(revision|n)\}`)
	for _, lang := range []string{"de", "en"} {
		for key, text := range readEasyDragLang(t, lang) {
			if strings.Contains(strings.ToLower(placeholder.ReplaceAllString(text, "")), "revision") {
				t.Errorf("%s %s says Revision: %q", lang, key, text)
			}
			if numbered.MatchString(text) {
				t.Errorf("%s %s says Version <number>, want v<number>: %q", lang, key, text)
			}
		}
	}
	en := readEasyDragLang(t, "en")
	for _, lang := range easyDragPluralLocalesAll() {
		words := readEasyDragLang(t, lang)
		for _, k := range easyDragVersionKeys {
			key := "easydrag.ui." + k
			want := easyDragPlaceholder.FindAllString(en[key], -1)
			for _, p := range want {
				if !strings.Contains(words[key], p) {
					t.Errorf("%s %s lost the placeholder %s", lang, key, p)
				}
			}
		}
	}
}

func easyDragPluralLocalesAll() []string {
	langs := make([]string, 0, len(easyDragPluralForms))
	for lang := range easyDragPluralForms {
		langs = append(langs, lang)
	}
	return langs
}
