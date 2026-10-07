package flows

import (
	"sort"
	"strings"
	"testing"
	"time"
)

func TestTemplatesBuildValidDrafts(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	tr := func(key string) string { return "T(" + key + ")" }
	infos := Templates()
	if len(infos) != 6 {
		t.Fatalf("templates = %d, want 6", len(infos))
	}
	vc := ValidateContext{Now: time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC), Location: time.UTC}
	for _, info := range infos {
		f, err := TemplateFlow(info.ID, tr)
		if err != nil {
			t.Fatalf("%s: %v", info.ID, err)
		}
		if f.Name != "T("+info.NameKey+")" || len(f.Nodes) < 2 || len(f.Edges) < 1 {
			t.Fatalf("%s: flow = %+v", info.ID, f)
		}
		vc.Mode = ModeDraft
		if issues := Validate(f, reg, vc); HasErrors(issues) {
			t.Errorf("%s: draft errors %+v", info.ID, issues)
		}
		vc.Mode = ModePublish
		for _, is := range Validate(f, reg, vc) {
			if is.Severity == SeverityError && is.Code != IssueParamRequired {
				t.Errorf("%s: unexpected publish error %+v", info.ID, is)
			}
		}
		again, _ := TemplateFlow(info.ID, tr)
		if again.Nodes[0].ID == f.Nodes[0].ID {
			t.Errorf("%s: every instance needs fresh node ids", info.ID)
		}
	}
	if _, err := TemplateFlow("nope", tr); err == nil {
		t.Fatal("unknown template must fail")
	}
}

func TestAINewsTemplateIsPublishable(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	f, err := TemplateFlow("ai_news_pdf_telegram", func(k string) string { return k })
	if err != nil {
		t.Fatal(err)
	}
	issues := Validate(f, reg, ValidateContext{Mode: ModePublish, Now: time.Now(), Location: time.UTC})
	if HasErrors(issues) {
		t.Fatalf("the AI news template must be publishable as is: %+v", issues)
	}
	if !strings.Contains(Stringify(f.NodeByKey("summary").Params["prompt"]), "{{search.results") {
		t.Fatal("the summary prompt must reference the search results")
	}
}

func TestCatalogI18nKeys(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	RefreshGenericTools(reg, []GenericTool{{Name: "docker", Schema: sampleToolSchema()}}, nil)
	keys := CatalogI18nKeys(reg)
	if !sort.StringsAreSorted(keys) {
		t.Fatal("keys must be sorted")
	}
	set := map[string]bool{}
	for _, k := range keys {
		if !strings.HasPrefix(k, "easydrag.") {
			t.Fatalf("unexpected key %q", k)
		}
		set[k] = true
	}
	for _, want := range []string{
		"easydrag.node.web_search.label", "easydrag.param.search_query", "easydrag.option.search_auto",
		"easydrag.category.trigger", "easydrag.template.ai_news_pdf_telegram.name", "easydrag.template.ai_news_pdf_telegram.text_1",
		"easydrag.node.trigger_schedule.summary", "easydrag.help.min_interval_seconds",
	} {
		if !set[want] {
			t.Errorf("missing key %s", want)
		}
	}
	for k := range set {
		if strings.Contains(k, "tool.docker") {
			t.Fatalf("generic tools have no i18n keys: %s", k)
		}
	}
}
