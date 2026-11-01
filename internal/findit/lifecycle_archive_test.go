package findit

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

func createProjectForItem(t *testing.T, store *Store, itemID string) Project {
	t.Helper()
	result, err := executeTest(t, store, "create", "projects", "", 0, "", map[string]any{
		"name": "Archive reference", "lines": []map[string]any{{"item_id": itemID, "quantity": "1"}},
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	return result.(Project)
}

func TestArchiveItemRespectsActiveProjectBOMReferences(t *testing.T) {
	for _, status := range []string{"draft", "released"} {
		t.Run(status, func(t *testing.T) {
			store := openTestStore(t)
			item := createTestItem(t, store, "BOM part", "BOM-"+status, "piece", false, nil)
			project := createProjectForItem(t, store, item.ID)
			if status == "released" {
				result, err := executeTest(t, store, "project_status", "projects", project.ID, project.Version, "release-"+status, map[string]any{"status": "released"})
				if err != nil {
					t.Fatalf("release project: %v", err)
				}
				project = result.(Project)
			}

			if _, err := executeTest(t, store, "archive", "items", item.ID, item.Version, "archive-active-bom-"+status, nil); ErrorCode(err) != "conflict" {
				t.Fatalf("archive item referenced by %s project: error = %v, want conflict", status, err)
			}

			_, err := executeTest(t, store, "project_status", "projects", project.ID, project.Version, "cancel-"+status, map[string]any{"status": "cancelled"})
			if err != nil {
				t.Fatalf("cancel project: %v", err)
			}
			if _, err := executeTest(t, store, "archive", "items", item.ID, item.Version, "archive-terminal-bom-"+status, nil); err != nil {
				t.Fatalf("archive item after project became terminal: %v", err)
			}
		})
	}
}

func TestReleaseProjectRejectsArchivedBOMItems(t *testing.T) {
	store := openTestStore(t)
	item := createTestItem(t, store, "Archived BOM part", "BOM-ARCHIVED", "piece", false, nil)
	project := createProjectForItem(t, store, item.ID)
	if _, err := store.db.Exec(`UPDATE items SET archived=1 WHERE id=?`, item.ID); err != nil {
		t.Fatalf("archive fixture item: %v", err)
	}

	if _, err := executeTest(t, store, "project_status", "projects", project.ID, project.Version, "release-archived-item", map[string]any{"status": "released"}); ErrorCode(err) != "conflict" {
		t.Fatalf("release project with archived BOM item error = %v, want conflict", err)
	}
	got, err := store.Query(context.Background(), QueryRequest{Operation: "get", Entity: "projects", ID: project.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got.(Project).Status != "draft" {
		t.Fatalf("project status after rejected release = %q, want draft", got.(Project).Status)
	}
}

func TestCustomTemplateCanArchiveWithItemsAndRetainsExistingAssignments(t *testing.T) {
	store := openTestStore(t)
	created, err := executeTest(t, store, "create", "templates", "", 0, "create-archiveable-template", map[string]any{
		"name": "Archived module", "category": "electronic_component",
		"fields": []map[string]any{{"key": "value", "label": "Value", "type": "text"}},
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	template := created.(Template)
	created, err = executeTest(t, store, "create", "items", "", 0, "create-archived-template-item", map[string]any{
		"name": "Existing module", "template_id": template.ID, "unit": "piece", "properties": map[string]any{"value": "before"},
	})
	if err != nil {
		t.Fatalf("create template item: %v", err)
	}
	item := created.(Item)
	if _, err := executeTest(t, store, "archive", "templates", template.ID, template.Version, "archive-used-template", nil); err != nil {
		t.Fatalf("archive used custom template: %v", err)
	}

	updated, err := executeTest(t, store, "update", "items", item.ID, item.Version, "edit-item-with-archived-template", map[string]any{
		"name": "Existing module revised", "unit": "piece", "properties": map[string]any{"value": "after"},
	})
	if err != nil {
		t.Fatalf("edit item while retaining its archived template: %v", err)
	}
	if got := updated.(Item); got.TemplateID != template.ID || got.Properties["value"] != "after" {
		t.Fatalf("updated item lost archived template assignment or property: %#v", got)
	}

	for _, test := range []struct {
		name string
		call func() error
	}{
		{name: "create new item", call: func() error {
			_, err := executeTest(t, store, "create", "items", "", 0, "new-item-archived-template", map[string]any{
				"name": "New module", "template_id": template.ID, "unit": "piece",
			})
			return err
		}},
		{name: "reassign existing item", call: func() error {
			other, err := executeTest(t, store, "create", "items", "", 0, "create-item-for-archived-assignment", map[string]any{
				"name": "Other module", "template_id": "generic", "unit": "piece",
			})
			if err != nil {
				return err
			}
			_, err = executeTest(t, store, "update", "items", other.(Item).ID, other.(Item).Version, "assign-archived-template", map[string]any{
				"name": "Other module", "template_id": template.ID, "unit": "piece",
			})
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); ErrorCode(err) != "not_found" {
				t.Fatalf("archived template use error = %v, want not_found", err)
			}
		})
	}
}

func TestArchivedItemsKeepTemplateFieldsAndCategoryInUse(t *testing.T) {
	store := openTestStore(t)
	created, err := executeTest(t, store, "create", "templates", "", 0, "create-retained-template", map[string]any{
		"name": "Retained module", "category": "electronic_component",
		"fields": []map[string]any{{"key": "value", "label": "Value", "type": "text"}},
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	template := created.(Template)
	created, err = executeTest(t, store, "create", "items", "", 0, "create-retained-template-item", map[string]any{
		"name": "Archived module", "template_id": template.ID, "unit": "piece", "properties": map[string]any{"value": "stored"},
	})
	if err != nil {
		t.Fatalf("create item: %v", err)
	}
	item := created.(Item)
	if _, err := executeTest(t, store, "archive", "items", item.ID, item.Version, "archive-retained-template-item", nil); err != nil {
		t.Fatalf("archive item fixture: %v", err)
	}

	for _, test := range []struct {
		name     string
		category string
		fields   []map[string]any
	}{
		{name: "field type", category: "electronic_component", fields: []map[string]any{{"key": "value", "label": "Value", "type": "number"}}},
		{name: "category", category: "cable", fields: []map[string]any{{"key": "value", "label": "Value", "type": "text"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			requestID := "change-retained-template-type"
			if test.category != "electronic_component" {
				requestID = "change-retained-template-category"
			}
			_, err := executeTest(t, store, "update", "templates", template.ID, template.Version, requestID, map[string]any{
				"name": template.Name, "category": test.category, "fields": test.fields,
			})
			if ErrorCode(err) != "conflict" {
				t.Fatalf("change template used by archived item error = %v, want conflict", err)
			}
		})
	}
}

func TestSeedPreservesUsedBuiltInDefinitionsOnIncompatibleCatalogChange(t *testing.T) {
	for _, test := range []struct {
		name       string
		fields     []PropertyDefinition
		properties map[string]any
		compatible bool
	}{
		{name: "compatible additive update", fields: func() []PropertyDefinition {
			for _, template := range Catalog().Templates {
				if template.ID == "resistor" {
					for _, field := range template.Fields {
						if field.Key == "resistance_ohms" {
							return []PropertyDefinition{field}
						}
					}
				}
			}
			return nil
		}(), properties: map[string]any{"resistance_ohms": 1000}, compatible: true},
		{name: "incompatible archived history", fields: []PropertyDefinition{{Key: "legacy_value", Label: "Legacy value", Type: "text"}}, properties: map[string]any{"legacy_value": "retained"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := openTestStore(t)
			oldFields, err := json.Marshal(test.fields)
			if err != nil {
				t.Fatal(err)
			}
			oldCategory := "cable"
			if !test.compatible {
				oldCategory = "other"
			}
			if _, err := store.db.Exec(`UPDATE templates SET category=?,fields_json=? WHERE id='resistor'`, oldCategory, string(oldFields)); err != nil {
				t.Fatalf("prepare prior catalog template: %v", err)
			}
			created, err := executeTest(t, store, "create", "items", "", 0, "create-built-in-history-item", map[string]any{
				"name": "Prior catalog item", "template_id": "resistor", "manufacturer": "Yageo", "part_number": "OLD-CATALOG",
				"unit": "piece", "properties": test.properties,
			})
			if err != nil {
				t.Fatalf("create prior catalog item: %v", err)
			}
			if !test.compatible {
				item := created.(Item)
				if _, err := executeTest(t, store, "archive", "items", item.ID, item.Version, "archive-old-catalog-item", nil); err != nil {
					t.Fatalf("archive prior catalog item: %v", err)
				}
			}

			if err := seedBuiltInTemplates(store.db); err != nil {
				t.Fatalf("refresh built-in templates: %v", err)
			}
			var category, fieldsJSON string
			if err := store.db.QueryRow(`SELECT category,fields_json FROM templates WHERE id='resistor'`).Scan(&category, &fieldsJSON); err != nil {
				t.Fatal(err)
			}
			if category != oldCategory {
				t.Fatalf("used built-in category = %q, want preserved %q", category, oldCategory)
			}
			var gotFields, wantFields []PropertyDefinition
			if err := json.Unmarshal([]byte(fieldsJSON), &gotFields); err != nil {
				t.Fatal(err)
			}
			if test.compatible {
				for _, template := range Catalog().Templates {
					if template.ID == "resistor" {
						wantFields = template.Fields
					}
				}
			} else {
				wantFields = test.fields
			}
			if !reflect.DeepEqual(gotFields, wantFields) {
				t.Fatalf("seeded fields = %#v, want %#v", gotFields, wantFields)
			}
		})
	}
}
