package ui

import (
	"encoding/json"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"aurago/internal/cydcreator"
)

func readCYDCreatorLang(t *testing.T, lang string) map[string]string {
	t.Helper()
	out := map[string]string{}
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/cyd_creator/"+lang+".json")), &out); err != nil {
		t.Fatalf("lang/cyd_creator/%s.json: %v", lang, err)
	}
	return out
}

var cydDesignerKeyLiteral = regexp.MustCompile(`'(cyd_creator\.[A-Za-z0-9_.]*)'(\s*\+)?`)
var cydPlaceholder = regexp.MustCompile(`\{[a-z_]+\}`)

// cydDesignerFallback matches t.or('cyd_creator.…', 'English fallback' with a plain string literal.
var cydDesignerFallback = regexp.MustCompile(`t\.or\('(cyd_creator\.[A-Za-z0-9_.]+)',\s*'((?:[^'\\]|\\.)*)'`)

// cydDesignerOwnedPrefixes are the key families only the designer defines (plan 4b); the shell
// reads theme names and board states with t.or fallbacks. orientation.* belongs to plan 4a.
var cydDesignerOwnedPrefixes = []string{"cyd_creator.designer.", "cyd_creator.widget.", "cyd_creator.group.", "cyd_creator.prop.",
	"cyd_creator.event.", "cyd_creator.action.", "cyd_creator.transition.", "cyd_creator.theme.", "cyd_creator.board.status_", "cyd_creator.issue."}

// TestCYDCreatorDesignerKeysExistInAllLocales checks every literal key of the designer modules and
// every member of the dynamic key families (catalog types, groups, props, events, enum options,
// themes, board states, issue codes) in all sixteen locales, with the same placeholders as English,
// and German without the formal address. The reverse checks keep no family the modules no longer
// build and no designer-owned key they no longer use; every literal t.or fallback equals the English
// bundle.
func TestCYDCreatorDesignerKeysExistInAllLocales(t *testing.T) {
	used := map[string]bool{}
	prefixes := map[string]bool{}
	fallbacks := map[string][]string{}
	for _, name := range cydDesignerScripts {
		text := readDesktopAssetText(t, "js/desktop/apps/"+name)
		for _, m := range cydDesignerKeyLiteral.FindAllStringSubmatch(text, -1) {
			if m[2] != "" || strings.HasSuffix(m[1], "_") || strings.HasSuffix(m[1], ".") {
				prefixes[m[1]] = true
			} else {
				used[m[1]] = true
			}
		}
		for _, m := range cydDesignerFallback.FindAllStringSubmatch(text, -1) {
			fallbacks[m[1]] = append(fallbacks[m[1]], strings.NewReplacer(`\'`, `'`, `\\`, `\`).Replace(m[2]))
		}
	}
	if len(used) < 100 {
		t.Fatalf("found only %d literal cyd_creator keys in the designer; is the scan broken?", len(used))
	}
	if len(fallbacks) < 50 {
		t.Fatalf("found only %d t.or fallbacks in the designer; is the scan broken?", len(fallbacks))
	}
	themes, err := cydcreator.Themes()
	if err != nil {
		t.Fatal(err)
	}
	families := map[string][]string{
		"cyd_creator.icons.":          strings.Fields("title loading insert locked budget_error inserted search style tone animation still animated size category all pixels memory memory_hint empty cancel style_playful style_modern style_comic style_sober tone_color tone_mono category_navigation category_status category_system category_home category_weather category_media"),
		"cyd_creator.widget.":         {},
		"cyd_creator.group.":          {},
		"cyd_creator.prop.":           {"bg_color", "bg_opacity", "text_color", "accent_color", "border_color", "border_width", "radius"},
		"cyd_creator.event.":          {},
		"cyd_creator.action.":         {"goto_screen", "back", "open_wifi_setup", "logic"},
		"cyd_creator.transition.":     {"none", "fade", "slide_left", "slide_right", "slide_up", "slide_down"},
		"cyd_creator.theme.":          {},
		"cyd_creator.board.status_":   {cydcreator.BoardVerified, cydcreator.BoardCommunity},
		"cyd_creator.issue.":          cydcreator.IssueCodes(),
		"cyd_creator.orientation.":    {"landscape", "portrait", "landscape_flipped", "portrait_flipped"},
		"cyd_creator.designer.align_": {"left", "hcenter", "right", "top", "vcenter", "bottom"},
		"cyd_creator.designer.z_":     {"front", "forward", "backward", "back"},
		"cyd_creator.designer.font_":  cydcreator.FontFamilies,
		"cyd_creator.designer.wifi_":  {"off", "optional", "required"},
		"cyd_creator.designer.opt_":   {},
	}
	for _, spec := range cydcreator.WidgetCatalog() {
		families["cyd_creator.widget."] = append(families["cyd_creator.widget."], spec.Type)
		families["cyd_creator.group."] = append(families["cyd_creator.group."], spec.Group)
		families["cyd_creator.event."] = append(families["cyd_creator.event."], spec.Events...)
		for _, p := range spec.Props {
			families["cyd_creator.prop."] = append(families["cyd_creator.prop."], p.Name)
			for _, o := range p.Options {
				families["cyd_creator.designer.opt_"] = append(families["cyd_creator.designer.opt_"], p.Name+"_"+o)
			}
		}
	}
	for _, th := range themes {
		families["cyd_creator.theme."] = append(families["cyd_creator.theme."], th.ID)
	}
	for _, motif := range cydcreator.Icons().Motifs {
		families["cyd_creator.icons."] = append(families["cyd_creator.icons."], "motif."+motif.ID)
	}
	for prefix := range prefixes {
		if _, ok := families[prefix]; !ok {
			t.Errorf("dynamic key family %s has no member list in this test", prefix)
		}
	}
	for prefix, members := range families {
		if !prefixes[prefix] {
			t.Errorf("key family %s is listed in this test, but no designer module builds keys from it", prefix)
		}
		for _, member := range members {
			used[prefix+member] = true
		}
	}
	keys := make([]string, 0, len(used))
	for key := range used {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	en := readCYDCreatorLang(t, "en")
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		words := en
		if lang != "en" {
			words = readCYDCreatorLang(t, lang)
		}
		for _, key := range keys {
			value := strings.TrimSpace(words[key])
			if value == "" {
				t.Errorf("lang/cyd_creator/%s.json misses %s", lang, key)
				continue
			}
			want := cydPlaceholder.FindAllString(en[key], -1)
			got := cydPlaceholder.FindAllString(value, -1)
			slices.Sort(want)
			slices.Sort(got)
			if !slices.Equal(want, got) {
				t.Errorf("lang/cyd_creator/%s.json %s has placeholders %v, English has %v", lang, key, got, want)
			}
		}
		for key := range words {
			for _, prefix := range cydDesignerOwnedPrefixes {
				if strings.HasPrefix(key, prefix) && !used[key] {
					t.Errorf("lang/cyd_creator/%s.json has %s, which the designer no longer uses", lang, key)
				}
			}
		}
	}
	for key, list := range fallbacks {
		for _, fallback := range list {
			if fallback != en[key] {
				t.Errorf("t.or fallback of %s is %q, the English bundle has %q", key, fallback, en[key])
			}
		}
	}
	de := readCYDCreatorLang(t, "de")
	formal := regexp.MustCompile(`\b(Sie|Ihnen|Ihre[nmrs]?)\b`)
	for _, key := range keys {
		if formal.MatchString(de[key]) {
			t.Errorf("lang/cyd_creator/de.json %s uses the formal address: %q", key, de[key])
		}
	}
}
