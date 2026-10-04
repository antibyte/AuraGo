package flows

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// Tests of the list of starter templates and of TemplateFlow itself (the unknown id, a
// translator without translations). They use the helpers of the hardening tests.

// The gallery lists the six templates in the order of the plan, each with categories of the
// catalog; the list is a copy that a caller may modify.
func TestTemplatesListInGalleryOrder(t *testing.T) {
	want := []string{"ai_news_pdf_telegram", "webhook_summary_email", "appointment_reminder", "leaving_home", "budget_guard", "rss_digest"}
	var ids []string
	for _, info := range Templates() {
		ids = append(ids, info.ID)
		if len(info.Categories) == 0 {
			t.Errorf("%s has no category", info.ID)
		}
		for _, c := range info.Categories {
			if !slices.Contains(CategoryOrder, c) {
				t.Errorf("%s: %q is not a category of the catalog %v", info.ID, c, CategoryOrder)
			}
		}
	}
	if !slices.Equal(ids, want) {
		t.Fatalf("templates = %v, want %v", ids, want)
	}

	first := Templates()
	first[0].Categories[0] = "changed"
	first[0].textKeys[0] = "changed"
	first[0].ID = "changed"
	again := Templates()
	if again[0].ID != want[0] || again[0].Categories[0] != "trigger" || again[0].textKeys[0] != templateKey(want[0], "text_1") {
		t.Errorf("a change of the result reached the list: %+v", again[0])
	}
}

// An id that is not a template is ErrUnknownTemplate, and the id, which a caller took from a
// request, is echoed cut like every other user text in an error.
func TestTemplateFlowUnknownTemplate(t *testing.T) {
	for _, id := range []string{"nope", "", "ai_news", strings.Repeat("x", 1000), strings.Repeat("ä", 1000), "a\nb"} {
		f, err := TemplateFlow(id, nil)
		if f != nil || !errors.Is(err, ErrUnknownTemplate) {
			t.Errorf("id %.20q: flow %v, error %v; want nil and ErrUnknownTemplate", id, f, err)
			continue
		}
		msg := err.Error()
		if n := utf8.RuneCountInString(msg); n > len(ErrUnknownTemplate.Error())+60 {
			t.Errorf("id %.20q: the message has %d runes", id, n)
		}
		if utf8.RuneCountInString(id) == 1000 && (!strings.Contains(msg, "…") || strings.Contains(msg, string([]rune(id)[:60]))) {
			t.Errorf("a long id is not cut: %.120q", msg)
		}
		if strings.Contains(msg, "\n") {
			t.Errorf("a line break of the id reached the message: %q", msg)
		}
	}
	if _, err := TemplateFlow("rss_digest", nil); err != nil {
		t.Errorf("a template is unknown: %v", err)
	}
}

// A translator without translations returns "" (or the key). The flow still has a name, a
// label for every node and a text in every parameter, and it passes draft validation: a
// missing translation must not make the template unusable.
func TestTemplateFlowWithoutTranslations(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	for name, tr := range map[string]func(string) string{
		"blank": func(string) string { return "" },
		"keys":  tplIdentity,
		"nil":   nil,
	} {
		for _, info := range Templates() {
			f, err := TemplateFlow(info.ID, tr)
			if err != nil {
				t.Fatalf("%s %s: %v", name, info.ID, err)
			}
			if f.Name != info.ID || f.Description != info.ID {
				t.Errorf("%s %s: name %q, description %q, want the template id", name, info.ID, f.Name, f.Description)
			}
			for _, n := range f.Nodes {
				if n.Label != n.Key {
					t.Errorf("%s %s: label of %s is %q, want the node key", name, info.ID, n.Key, n.Label)
				}
			}
			params := fmt.Sprint(func() []any {
				var all []any
				for _, n := range f.Nodes {
					all = append(all, n.Params)
				}
				return all
			}())
			for _, k := range info.textKeys {
				if !strings.Contains(params, k) {
					t.Errorf("%s %s: no parameter holds %s (a blank text): %s", name, info.ID, k, params)
				}
			}
			vc := ValidateContext{Mode: ModeDraft, Now: time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC), Location: time.UTC}
			for _, is := range Validate(f, reg, vc) {
				if is.Severity == SeverityError || is.Code == IssueNameRequired {
					t.Errorf("%s %s: %+v", name, info.ID, is)
				}
			}
		}
	}
}
