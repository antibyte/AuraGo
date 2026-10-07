package flows

import (
	"slices"
	"sort"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// Hardening tests of the translations in ui/lang/easydrag. catalog_i18n_test.go holds the
// plan's tests; the helpers here start with i18n.

// i18nTranslator translates with one locale file and returns the key for what the file
// lacks, like the translator of TestTemplatesAreValidInEveryLocale.
func i18nTranslator(m map[string]string) func(string) string {
	return func(key string) string {
		if v, ok := m[key]; ok {
			return v
		}
		return key
	}
}

// i18nParamSpec returns the parameter of def with the given name.
func i18nParamSpec(def *NodeDef, name string) (ParamSpec, bool) {
	for _, p := range def.Params {
		if p.Name == name {
			return p, true
		}
	}
	return ParamSpec{}, false
}

// i18nSortedExpressions lists "<node key>.<param>: <expression>" for every template
// expression of f (tplExpressions), sorted, so that a language that puts the amounts of the
// budget message in another order still compares equal.
func i18nSortedExpressions(t *testing.T, f *Flow) []string {
	t.Helper()
	exprs := tplExpressions(t, f)
	sort.Strings(exprs)
	return exprs
}

// A translated template text is trusted template text, so a broken expression or a
// misspelled filter in one language would reach the flow. In a draft that is only a
// TEMPLATE_SYNTAX or TEMPLATE_UNKNOWN_ROOT warning, which TestTemplatesAreValidInEveryLocale
// does not see. So no template may get a TEMPLATE_* issue of any severity in any language,
// every parameter must parse, a parameter that holds an expression must be templatable (or
// the expression would stay literal text), and every language must keep exactly the
// expressions of the English texts, in the same parameters.
func TestTemplateTranslationsHaveNoTemplateIssues(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	vc := ValidateContext{Mode: ModeDraft, Now: time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC), Location: time.UTC}
	en := i18nTranslator(loadEasyDragLocale(t, "en"))
	for _, lang := range catalogLocales {
		tr := i18nTranslator(loadEasyDragLocale(t, lang))
		for _, info := range Templates() {
			f := tplBuild(t, info.ID, tr)
			for _, is := range Validate(f, reg, vc) {
				if strings.HasPrefix(is.Code, "TEMPLATE_") {
					t.Errorf("%s/%s: %+v", lang, info.ID, is)
				}
			}
			for i := range f.Nodes {
				n := &f.Nodes[i]
				def := lookupDef(t, reg, n.Type)
				for _, name := range sortedKeys(n.Params) {
					refs, problems := CollectTemplateRefs(map[string]any{name: n.Params[name]})
					if len(problems) > 0 {
						t.Errorf("%s/%s %s.%s: template problems %v", lang, info.ID, n.Key, name, problems)
					}
					if spec, ok := i18nParamSpec(def, name); len(refs) > 0 && (!ok || !spec.Templatable) {
						t.Errorf("%s/%s %s.%s holds an expression but is not templatable", lang, info.ID, n.Key, name)
					}
				}
			}
			want := i18nSortedExpressions(t, tplBuild(t, info.ID, en))
			if got := i18nSortedExpressions(t, f); !slices.Equal(got, want) {
				t.Errorf("%s/%s: expressions %v, want those of en %v", lang, info.ID, got, want)
			}
		}
	}
}

// Every trigger.data field a translated text reads must be in the sample of its trigger,
// built with the template's own parameters, and every other reference must name a field the
// node before it outputs. The check is the one of TestTemplateReferencesNameRealFields, run
// with each language file instead of the fixture tplShipped.
func TestTemplateTranslationsReadRealFields(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	known := map[string][]string{
		"run":  {"id", "started_at", "mode", "revision"},
		"flow": {"id", "name"},
	}
	triggerOutput := []string{"data", "fired_at", "type", "node"}
	for _, lang := range catalogLocales {
		// 9 expressions in the builders and 4 in the reminder and budget texts.
		tplCheckReferences(t, reg, lang, i18nTranslator(loadEasyDragLocale(t, lang)), 13, known, triggerOutput)
	}
}

// The AI templates append the data after a blank line, and the data can be empty (no search
// results, no feed items, an empty webhook body). Each instruction therefore tells the model
// what to answer then, so it does not invent content. Every language has its own wording:
// each text is a single paragraph and is translated, not the English one.
func TestTemplateAIInstructionsAreTranslated(t *testing.T) {
	keys := []string{
		"easydrag.template.ai_news_pdf_telegram.text_2",
		"easydrag.template.rss_digest.text_1",
		"easydrag.template.webhook_summary_email.text_1",
	}
	en := loadEasyDragLocale(t, "en")
	for _, lang := range catalogLocales {
		m := loadEasyDragLocale(t, lang)
		for _, key := range keys {
			v := m[key]
			switch {
			case strings.TrimSpace(v) == "":
				t.Errorf("%s.json: %s is empty", lang, key)
			case strings.ContainsAny(v, "\r\n"):
				t.Errorf("%s.json: %s is not a single paragraph: %q", lang, key, v)
			case lang != "en" && v == en[key]:
				t.Errorf("%s.json: %s is the English text", lang, key)
			}
		}
	}
}

// Labels are shown in narrow places of the editor (the palette, a node on the canvas, a
// segmented control), so a translation of a short English label stays short: at most 30
// characters when the English label has 20 or fewer. Labels are the node labels and the
// option, parameter and category labels.
func TestCatalogLabelTranslationsStayShort(t *testing.T) {
	isLabel := func(key string) bool {
		if strings.HasSuffix(key, ".label") {
			return true
		}
		for _, prefix := range []string{"easydrag.option.", "easydrag.param.", "easydrag.category.", "easydrag.tool_category."} {
			if strings.HasPrefix(key, prefix) {
				return true
			}
		}
		return false
	}
	en := loadEasyDragLocale(t, "en")
	checked := 0
	for _, lang := range catalogLocales {
		m := loadEasyDragLocale(t, lang)
		for key, value := range en {
			if !isLabel(key) || utf8.RuneCountInString(value) > 20 {
				continue
			}
			checked++
			if n := utf8.RuneCountInString(m[key]); n > 30 {
				t.Errorf("%s.json %s: %q has %d characters, at most 30 (en %q)", lang, key, m[key], n, value)
			}
		}
	}
	// A guard against an empty walk: the catalog has far more than 100 short labels.
	if checked < 100*len(catalogLocales) {
		t.Fatalf("only %d labels checked", checked)
	}
}
