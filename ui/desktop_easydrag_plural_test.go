package ui

import (
	"regexp"
	"strings"
	"testing"
)

// easyDragPluralForms lists, per locale, the plural categories besides "other" that
// Intl.PluralRules gives for the whole numbers 0 to 10000. scripts/test-easydrag-format.mjs
// compares this table with Intl. A count text needs a key_<category> text for each entry;
// the bare key is the "other" form.
var easyDragPluralForms = map[string][]string{
	"cs": {"one", "few"},
	"da": {"one"},
	"de": {"one"},
	"el": {"one"},
	"en": {"one"},
	"es": {"one"},
	"fr": {"one"},
	"hi": {"one"},
	"it": {"one"},
	"ja": {},
	"nl": {"one"},
	"no": {"one"},
	"pl": {"one", "few", "many"},
	"pt": {"one"},
	"sv": {"one"},
	"zh": {},
}

// easyDragPluralKeys are the count texts whose words agree with the count (noun, adjective, verb).
// They are read only through ED.format.plural and have the forms of every locale.
var easyDragPluralKeys = []string{
	"folder_flows", "group_member_count", "group_edges_count", "group_errors", "group_running",
	"group_success", "group_disabled", "cred_purge_versions", "cred_purge_pinned", "versions_connections",
}

// easyDragNumberNeutralKeys are count texts with no word that agrees with the count in any locale: the
// number follows a colon or sits in brackets or in a pair. announce_*_many are used from 2 only.
var easyDragNumberNeutralKeys = []string{
	"announce_added_many", "announce_deleted_many", "announce_disabled_many", "announce_enabled_many", "announce_moved_many",
	"copied", "diff_added", "diff_changed", "diff_removed", "names_more", "output_items", "output_tokens",
	"preview_list", "preview_object", "preview_text", "publish_blocked", "publish_warnings", "summary_items",
	"tree_more", "goal_counter", "batch_result_summary", "home_bulk_selected", "extraction_selected",
	"pack_blocks", "credential_move_text",
}

var (
	easyDragPluralSuffix = regexp.MustCompile(`_(zero|one|two|few|many)$`)
	easyDragPlaceholder  = regexp.MustCompile(`\{[a-z_]+\}`)
)

func TestEasyDragCountKeysHaveThePluralFormsOfEveryLocale(t *testing.T) {
	const prefix = "easydrag.ui."
	plural, listed := map[string]bool{}, map[string]bool{}
	for _, k := range easyDragPluralKeys {
		plural[prefix+k], listed[prefix+k] = true, true
	}
	for _, k := range easyDragNumberNeutralKeys {
		if plural[prefix+k] {
			t.Errorf("%s is listed as plural and as number-neutral", k)
		}
		listed[prefix+k] = true
	}
	en := readEasyDragLang(t, "en")
	for key, text := range en {
		if !strings.Contains(text, "{count}") {
			continue
		}
		if base := easyDragPluralSuffix.ReplaceAllString(key, ""); base != key && plural[base] {
			continue // a plural form of a listed key
		}
		if !listed[key] {
			t.Errorf("%s has {count} but is in neither easyDragPluralKeys nor easyDragNumberNeutralKeys", key)
		}
	}
	for lang, forms := range easyDragPluralForms {
		words := readEasyDragLang(t, lang)
		for _, k := range easyDragPluralKeys {
			key := prefix + k
			bare := words[key]
			if !strings.Contains(bare, "{count}") {
				t.Errorf("%s %s: the bare (other) form is missing or has no {count}", lang, key)
			}
			// ja and zh need no "one" form, but carry one (the bare text) to keep the key parity with en
			// that internal/flows checks: en has key_one, so every locale does.
			want := forms
			if !containsString(want, "one") {
				want = append(append([]string{}, want...), "one")
			}
			for _, cat := range want {
				form := strings.TrimSpace(words[key+"_"+cat])
				if form == "" {
					t.Errorf("%s needs %s_%s", lang, key, cat)
					continue
				}
				for _, p := range easyDragPlaceholder.FindAllString(form, -1) {
					if !strings.Contains(bare, p) {
						t.Errorf("%s %s_%s uses %s, which the bare form does not", lang, key, cat, p)
					}
				}
			}
		}
		for _, k := range easyDragNumberNeutralKeys {
			if !strings.Contains(words[prefix+k], "{count}") {
				t.Errorf("%s %s lost its {count}", lang, prefix+k)
			}
		}
	}
	if len(easyDragPluralForms) != 16 {
		t.Errorf("easyDragPluralForms covers %d locales, want 16", len(easyDragPluralForms))
	}
}
