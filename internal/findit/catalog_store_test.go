package findit

import (
	"context"
	"encoding/json"
	"testing"
)

func TestCatalogContainsCoreOpAmpAndMemorySpecifications(t *testing.T) {
	catalog := Catalog()
	templates := make(map[string]Template, len(catalog.Templates))
	for _, template := range catalog.Templates {
		templates[template.ID] = template
	}
	for _, test := range []struct {
		template string
		key      string
		typeName string
		unit     string
		required bool
	}{
		{"op_amp", "channels", "number", "", false},
		{"op_amp", "slew_rate_v_us", "number", "V/µs", false},
		{"op_amp", "rail_to_rail_input", "boolean", "", false},
		{"op_amp", "rail_to_rail_output", "boolean", "", false},
		{"memory", "capacity_gb", "number", "GB", false},
		{"memory", "memory_type", "text", "", false},
		{"memory", "form_factor", "text", "", false},
		{"memory", "speed_mts", "number", "MT/s", false},
		{"memory", "ecc", "boolean", "", false},
	} {
		template, ok := templates[test.template]
		if !ok {
			t.Fatalf("catalog missing template %q", test.template)
		}
		var field *PropertyDefinition
		for i := range template.Fields {
			if template.Fields[i].Key == test.key {
				field = &template.Fields[i]
				break
			}
		}
		if field == nil {
			t.Errorf("template %q is missing field %q", test.template, test.key)
			continue
		}
		if field.Type != test.typeName || field.Unit != test.unit || field.Required != test.required {
			t.Errorf("template %q field %q = {type:%q unit:%q required:%t}, want {type:%q unit:%q required:%t}",
				test.template, test.key, field.Type, field.Unit, field.Required, test.typeName, test.unit, test.required)
		}
	}
}

func TestMemoryTemplateAcceptsUnknownSpecsAsBlank(t *testing.T) {
	store := openTestStore(t)
	result, err := executeTest(t, store, "create", "items", "", 0, "memory-with-unknown-specs", map[string]any{
		"name": "Unidentified memory module", "template_id": "memory", "unit": "piece", "properties": map[string]any{},
	})
	if err != nil {
		t.Fatalf("create memory item with unknown optional specifications: %v", err)
	}
	item := result.(Item)
	if len(item.Properties) != 0 {
		t.Fatalf("unknown memory specifications should stay blank, got properties %#v", item.Properties)
	}
}

func TestStoreCatalogUsesPersistedBuiltInDefinitions(t *testing.T) {
	store := openTestStore(t)
	if _, err := executeTest(t, store, "create", "items", "", 0, "catalog-item", map[string]any{
		"name": "Legacy amplifier", "template_id": "op_amp", "unit": "piece",
		"properties": map[string]any{"supply_voltage_v": "5"},
	}); err != nil {
		t.Fatalf("create template item: %v", err)
	}
	legacyFields, err := json.Marshal([]PropertyDefinition{{Key: "legacy_gain", Label: "Legacy gain", Type: "text"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(context.Background(), `UPDATE templates SET category=?,fields_json=? WHERE id=?`,
		"other", string(legacyFields), "op_amp"); err != nil {
		t.Fatalf("seed retained template definition: %v", err)
	}

	catalog, err := store.Catalog(context.Background())
	if err != nil {
		t.Fatalf("Store.Catalog(): %v", err)
	}
	var opAmp *Template
	for i := range catalog.Templates {
		if catalog.Templates[i].ID == "op_amp" {
			opAmp = &catalog.Templates[i]
		}
	}
	if opAmp == nil {
		t.Fatal("store catalog is missing built-in op_amp")
	}
	if opAmp.Category != "other" || len(opAmp.Fields) != 1 || opAmp.Fields[0].Key != "legacy_gain" {
		t.Fatalf("catalog returned static op_amp definition instead of persisted one: %#v", opAmp)
	}
	for _, template := range catalog.Templates {
		if !template.BuiltIn {
			t.Errorf("Store.Catalog() returned non-built-in template %q", template.ID)
		}
	}
}

func TestStoreCatalogHonorsCancelledContext(t *testing.T) {
	store := openTestStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.Catalog(ctx); err == nil {
		t.Fatal("Store.Catalog() succeeded with a cancelled context")
	}
}
