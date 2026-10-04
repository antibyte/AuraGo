package flows

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

// CatalogI18nKeys lists exactly the keys DescribeNodeTypes asks the translator for.
func TestCatalogI18nKeys(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	RefreshGenericTools(reg, []GenericTool{{Name: "docker", Category: "infrastructure", Schema: sampleToolSchema()}}, StaticEnv{})
	asked := map[string]bool{}
	DescribeNodeTypes(reg, func(key string) string {
		asked[key] = true
		return key
	})
	keys := CatalogI18nKeys(reg)
	if !slices.IsSorted(keys) || len(slices.Compact(slices.Clone(keys))) != len(keys) {
		t.Fatalf("keys are not sorted and unique: %v", keys)
	}
	got := map[string]bool{}
	for _, k := range keys {
		got[k] = true
		if !asked[k] {
			t.Errorf("key %s is listed but never asked for", k)
		}
	}
	for k := range asked {
		if !got[k] {
			t.Errorf("key %s is asked for but not listed", k)
		}
	}
	for _, want := range []string{
		"easydrag.node.ai_step.label",         // a node label
		"easydrag.node.web_search.label",      // another one
		"easydrag.param.search_query",         // a parameter label
		"easydrag.option.search_auto",         // an option label
		"easydrag.help.ha_service",            // a help text
		"easydrag.param.field_name",           // inside a fields parameter
		"easydrag.node.trigger_webhook.label", // a trigger
	} {
		if !got[want] {
			t.Errorf("missing key %s", want)
		}
	}
	for _, k := range keys {
		if strings.Contains(k, "docker") || !strings.HasPrefix(k, "easydrag.") {
			t.Errorf("unexpected key %q", k)
		}
	}
	if keys := CatalogI18nKeys(nil); keys == nil || len(keys) != 0 {
		t.Errorf("nil registry: %#v", keys)
	}
}

// Keys of dynamic output fields count too, and a hook that panics does not stop the listing.
func TestCatalogI18nKeysFromHooks(t *testing.T) {
	reg := NewRegistry()
	reg.MustRegister(&NodeDef{Type: "test.dyn", Category: "test", LabelKey: "k.label",
		Params: []ParamSpec{{Name: "p", Kind: ParamFields, LabelKey: "k.p", HelpKey: "k.p.help",
			Options: []Option{{Value: "v", LabelKey: "k.opt"}},
			Fields:  []ParamSpec{{Name: "inner", LabelKey: "k.inner", Options: []Option{{Value: "w", LabelKey: "k.inner.opt"}}}}}},
		OutputFields:     []FieldSpec{{Name: "static", DescriptionKey: "k.static"}},
		OutputFieldsFunc: func(*Node) []FieldSpec { return []FieldSpec{{Name: "dynamic", DescriptionKey: "k.dynamic"}} }})
	reg.MustRegister(&NodeDef{Type: "test.panics", Category: "test", LabelKey: "k.panics", DescriptionKey: "k.panics.desc", SummaryKey: "k.panics.sum",
		OutputFieldsFunc: func(*Node) []FieldSpec { panic("boom") }})
	reg.MustRegister(&NodeDef{Type: GenericTypePrefix + "skipped", Category: "tool:x", LabelKey: "k.skipped"})
	want := []string{"k.dynamic", "k.inner", "k.inner.opt", "k.label", "k.opt", "k.p", "k.p.help", "k.panics", "k.panics.desc", "k.panics.sum", "k.static"}
	if got := CatalogI18nKeys(reg); !reflect.DeepEqual(got, want) {
		t.Errorf("keys = %v\nwant   %v", got, want)
	}
}
