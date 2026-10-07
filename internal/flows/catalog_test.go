package flows

import (
	"strings"
	"testing"
)

// fullEnv makes every tool used by curated nodes available.
func fullEnv() StaticEnv {
	env := StaticEnv{}
	for _, tool := range []string{"ddg_search", BraveSearchTool, "web_scraper", "api_request", "document_creator", GotenbergTool,
		PDFExtractorTool, "filesystem", "send_telegram", "send_email", "send_notification", "send_discord",
		"home_assistant", "mqtt_publish", "manage_appointments", "manage_todos"} {
		env[tool] = Availability{}
	}
	return env
}

func catalogRegistry(t *testing.T, env CatalogEnv) *Registry {
	t.Helper()
	reg := NewRegistry()
	if err := RegisterCatalog(reg, env); err != nil {
		t.Fatalf("RegisterCatalog: %v", err)
	}
	return reg
}

func TestRegisterCatalogCounts(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	if n := len(reg.All()); n != 35 {
		t.Fatalf("phase 1 catalog has %d node types, want 35 (13 triggers + 22 others)", n)
	}
	triggers := 0
	for _, def := range reg.All() {
		if def.Trigger {
			triggers++
		}
		if def.Execute == nil && !def.Trigger {
			t.Errorf("%s has no Execute", def.Type)
		}
		if def.LabelKey == "" {
			t.Errorf("%s has no label key", def.Type)
		}
	}
	if triggers != 13 {
		t.Fatalf("triggers = %d", triggers)
	}
}

func TestDescribeNodeTypes(t *testing.T) {
	reg := catalogRegistry(t, StaticEnv{"ddg_search": {}})
	RefreshGenericTools(reg, []GenericTool{{Name: "docker", Category: "infrastructure", Schema: sampleToolSchema()}}, StaticEnv{})
	tr := func(key string) string {
		switch key {
		case "easydrag.node.web_search.label":
			return "Websuche"
		case "easydrag.param.search_query":
			return "Suchbegriff"
		case "easydrag.option.search_auto":
			return "Automatisch"
		}
		return key
	}
	infos := DescribeNodeTypes(reg, tr)
	if infos[0].Category != "trigger" || infos[len(infos)-1].Type != GenericTypePrefix+"docker" {
		t.Fatalf("order: first %s, last %s", infos[0].Type, infos[len(infos)-1].Type)
	}
	byType := map[string]NodeTypeInfo{}
	for _, info := range infos {
		byType[info.Type] = info
	}
	search := byType[TypeWebSearch]
	if search.Label != "Websuche" || search.Params[0].Label != "Suchbegriff" || search.Params[2].Options[0].Label != "Automatisch" ||
		search.Params[2].Options[1].Label != "DuckDuckGo" || !search.Untrusted || search.Availability.State != AvailableState ||
		search.PrimaryInput != "query" || len(search.Inputs) != 1 || search.Outputs[0] != PortOut {
		t.Fatalf("web.search info = %+v", search)
	}
	if byType[TypeTelegram].Availability.State != NeedsSetupState {
		t.Fatal("telegram is not configured in this env")
	}
	if byType[TypeSwitch].DynamicOutputs != "cases" || byType[TypeAIStep].DynamicFields != "fields" {
		t.Fatal("dynamic markers missing")
	}
	hook := byType[TypeTriggerWebhook]
	if !hook.Trigger || len(hook.Inputs) != 0 || hook.Sample["payload"] == nil {
		t.Fatalf("webhook info = %+v", hook)
	}
	if ha := byType[TypeHomeAssistant]; len(ha.Effects) != 1 || ha.Effects[0] != EffectControlsDevices || ha.Risky {
		t.Fatalf("HA info = %+v", ha)
	}
	docker := byType[GenericTypePrefix+"docker"]
	if docker.Label != "Docker" || docker.Params[1].Label != "Name" || docker.Category != "tool:infrastructure" {
		t.Fatalf("docker info = %+v", docker)
	}
	if !strings.HasPrefix(byType[TypeIf].Label, "easydrag.node.") {
		t.Fatalf("untranslated labels fall back to the key, got %q", byType[TypeIf].Label)
	}
}
