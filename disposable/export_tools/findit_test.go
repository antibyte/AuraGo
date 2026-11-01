package main

import (
	"strings"
	"testing"
)

func TestFindItTrainingTasksDescribeReadOperationsInBothLanguages(t *testing.T) {
	cases := []struct {
		operation string
		entity    string
		args      map[string]interface{}
		wantDE    []string
		wantEN    []string
	}{
		{
			operation: "search",
			entity:    "items",
			args: map[string]interface{}{
				"query": "10k resistor", "template_id": "resistor",
				"filters": []interface{}{map[string]interface{}{"field": "resistance_ohms", "operator": "eq", "value": 10000.0}},
			},
			wantDE: []string{"physischen Inventar", "10k resistor", "resistor", "typisierten"},
			wantEN: []string{"physical inventory", "10k resistor", "resistor", "typed"},
		},
		{operation: "get", entity: "items", args: map[string]interface{}{"id": "item-demo-001"}, wantDE: []string{"FindIt-Eintrag", "Artikel", "item-demo-001"}, wantEN: []string{"item", "item-demo-001"}},
		{operation: "availability", entity: "items", args: map[string]interface{}{"id": "item-demo-001"}, wantDE: []string{"Lagerbestand", "reservierte", "frei verfügbare", "item-demo-001"}, wantEN: []string{"on-hand", "reserved", "free stock", "item-demo-001"}},
		{operation: "shortages", entity: "projects", args: map[string]interface{}{"id": "project-demo-001"}, wantDE: []string{"Fehlmengen", "Stückliste", "frei verfügbaren", "project-demo-001"}, wantEN: []string{"shortages", "BOM", "free stock", "project-demo-001"}},
	}
	for _, tc := range cases {
		t.Run(tc.operation, func(t *testing.T) {
			args := map[string]interface{}{"operation": tc.operation, "entity": tc.entity}
			for key, value := range tc.args {
				args[key] = value
			}
			choice := fixtureChoice{Tool: ToolExport{Name: "findit"}, Arguments: args}
			for _, check := range []struct {
				language string
				needles  []string
			}{{"de", tc.wantDE}, {"en", tc.wantEN}} {
				got := naturalTaskClause(choice, check.language)
				for _, needle := range check.needles {
					if !strings.Contains(got, needle) {
						t.Errorf("%s FindIt %s task %q is missing %q", check.language, tc.operation, got, needle)
					}
				}
				lower := strings.ToLower(got)
				for _, forbidden := range []string{"create", "delete", "update", "anlegen", "löschen", "aktualisieren"} {
					if strings.Contains(lower, forbidden) {
						t.Errorf("%s FindIt %s task describes a write: %q", check.language, tc.operation, got)
					}
				}
			}
		})
	}
}

func TestFindItTemplateTrainingTaskDiscoversPropertyCatalog(t *testing.T) {
	choice := fixtureChoice{Tool: ToolExport{Name: "findit"}, Arguments: map[string]interface{}{"operation": "search", "entity": "templates"}}
	for _, language := range []string{"de", "en"} {
		got := naturalTaskClause(choice, language)
		if !strings.Contains(got, "Vorlagen") && language == "de" {
			t.Errorf("German template task does not request the template catalog: %q", got)
		}
		if !strings.Contains(got, "templates") && language == "en" {
			t.Errorf("English template task does not request the template catalog: %q", got)
		}
	}
}
