package findit

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestAuditCSVExportEscapesFormulaCellsAndPreservesReimport(t *testing.T) {
	store := openTestStore(t)
	names := []string{"=1+1", "+SUM(1,2)", "-1+2", "@SUM(1)", "＝1+1", "'original", "ordinary"}
	for i, name := range names {
		createTestItem(t, store, name, fmt.Sprintf("CSV-%d", i), "piece", false, nil)
	}
	data, err := store.ExportCSV(context.Background(), "items", "")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(strings.NewReader(string(data))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		if strings.ContainsRune("=+-@＝＋－＠", []rune(row[0])[0]) {
			t.Fatalf("formula-active exported name: %q", row[0])
		}
	}
	target := openTestStore(t)
	if _, err := target.ImportCSV(context.Background(), ImportRequest{Entity: "items", CSV: string(data), RequestID: "roundtrip"}); err != nil {
		t.Fatal(err)
	}
	result, err := target.Query(context.Background(), QueryRequest{Operation: "search", Entity: "items"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, value := range result.(SearchResult).Items {
		got[value.(Item).Name] = true
	}
	for _, name := range names {
		if !got[name] {
			t.Errorf("CSV roundtrip changed %q", name)
		}
	}
}

func TestAuditCSVParserStopsAtRowLimit(t *testing.T) {
	// An invalid tail must never be read after the supported row budget.
	_, err := parseCSV("name\n" + strings.Repeat("item\n", maxCSVRows+1) + "\"unterminated")
	if ErrorCode(err) != "invalid" || !strings.Contains(err.Error(), "too many rows") {
		t.Fatalf("expected early row limit, got %v", err)
	}
}

func TestAuditSearchEngineeringUnitsAndArchivedTemplate(t *testing.T) {
	store := openTestStore(t)
	item := createTestItem(t, store, "Plain component", "Q-100", "piece", false, map[string]any{"resistance_ohms": json.Number("4700")})
	for _, query := range []string{"4k7", "4,7 kΩ", "4.7kOhm"} {
		result, err := store.Query(context.Background(), QueryRequest{Operation: "search", Entity: "items", Query: query})
		if err != nil || result.(SearchResult).Total != 1 {
			t.Errorf("query %q missed item %s: %#v, %v", query, item.ID, result, err)
		}
	}
	result, err := executeTest(t, store, "create", "templates", "", 0, "", map[string]any{"name": "Custom voltage", "category": "other", "fields": []map[string]any{{"key": "reading", "label": "Reading", "type": "number", "unit": "V"}}})
	if err != nil {
		t.Fatal(err)
	}
	template := result.(Template)
	created, err := executeTest(t, store, "create", "items", "", 0, "", map[string]any{"name": "Custom meter", "template_id": template.ID, "unit": "piece", "properties": map[string]any{"reading": 3}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = executeTest(t, store, "archive", "templates", template.ID, template.Version, "", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	result, err = store.Query(context.Background(), QueryRequest{Operation: "search", Entity: "items", TemplateID: template.ID, Filters: []PropertyFilter{{Field: "reading", Operator: "eq", Value: 3.0}}})
	if err != nil || result.(SearchResult).Total != 1 {
		t.Fatalf("archived template values became unsearchable: %#v %v", result, err)
	}
	meter := created.(Item)
	if _, err := executeTest(t, store, "archive", "items", meter.ID, meter.Version, "", nil); err != nil {
		t.Fatal(err)
	}
	result, err = store.Query(context.Background(), QueryRequest{Operation: "search", Entity: "items", TemplateID: template.ID, Filters: []PropertyFilter{{Field: "reading", Operator: "eq", Value: 3.0}}})
	if err != nil || result.(SearchResult).Total != 0 {
		t.Fatalf("historical template filter must remain valid without returning archived items: %#v %v", result, err)
	}
}

func TestAuditPropertyFilterRejectsAmbiguousUnits(t *testing.T) {
	store := openTestStore(t)
	for _, unit := range []string{"V", "mV"} {
		_, err := executeTest(t, store, "create", "templates", "", 0, "", map[string]any{"name": "Reading " + unit, "category": "other", "fields": []map[string]any{{"key": "reading", "label": "Reading", "type": "number", "unit": unit}}})
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err := store.Query(context.Background(), QueryRequest{Operation: "search", Entity: "items", Filters: []PropertyFilter{{Field: "reading", Operator: "eq", Value: 3.0}}})
	if ErrorCode(err) != "invalid" {
		t.Fatalf("mixed V/mV filter must require a template: %v", err)
	}
}

func TestAuditEngineeringPrefixCaseAndLegacyAliases(t *testing.T) {
	store := openTestStore(t)
	mega := createTestItem(t, store, "Large resistance", "BIG", "piece", false, map[string]any{"resistance_ohms": json.Number("4700000")})
	micro := createTestItem(t, store, "Small resistance", "SMALL", "piece", false, map[string]any{"resistance_ohms": json.Number("0.0000047")})
	terra := createTestItem(t, store, "Very large resistance", "HUGE", "piece", false, map[string]any{"resistance_ohms": json.Number("4700000000000")})
	// Preserve an old, incorrectly generated micro alias on the mega item.
	if _, err := store.db.Exec(`UPDATE findit_items_fts SET properties='resistance_ohms 4700000 4.7u 4u7' WHERE item_id=?`, mega.ID); err != nil {
		t.Fatal(err)
	}
	for query, id := range map[string]string{"4.7M": mega.ID, "4.7u": micro.ID, "4,7 MΩ": mega.ID, "4.7µΩ": micro.ID, "4.7T": terra.ID, "4.7 TΩ": terra.ID} {
		result, err := store.Query(context.Background(), QueryRequest{Operation: "search", Entity: "items", Query: query})
		if err != nil {
			t.Fatal(err)
		}
		items := result.(SearchResult).Items
		if len(items) != 1 || items[0].(Item).ID != id {
			t.Errorf("query %q confused SI prefixes: %#v", query, items)
		}
	}
}

func TestAuditRKMNumbersKeepMagnitudeAndPrefixCase(t *testing.T) {
	for input, want := range map[string]float64{"4m7": 0.0047, "4M7": 4700000, "100000000000000000000R0": 1e20} {
		got, err := parseNumeric(input)
		if err != nil || got != want {
			t.Errorf("parseNumeric(%q) = %v, %v; want %v", input, got, err, want)
		}
	}
	for _, input := range []string{strings.Repeat("9", 310) + "R1", strings.Repeat("9", 306) + "M1"} {
		if _, err := parseNumeric(input); ErrorCode(err) != "invalid" {
			t.Errorf("overflowing RKM number must be rejected: %v", err)
		}
	}
}

func TestAuditSearchCatalogLengthAndCapacityUnits(t *testing.T) {
	store := openTestStore(t)
	for _, fixture := range []struct {
		template, field, query string
		value                  float64
	}{
		{"cable", "length_m", "4.7 m", 4.7},
		{"connector", "pitch_mm", "4.7 mm", 4.7},
		{"memory", "capacity_gb", "16 GB", 16},
		{"battery", "capacity_ah", "4700 mAh", 4.7},
	} {
		created, err := executeTest(t, store, "create", "items", "", 0, "", map[string]any{
			"name": "Measured " + fixture.template, "template_id": fixture.template, "unit": "piece", "properties": map[string]any{fixture.field: fixture.value},
		})
		if err != nil {
			t.Fatal(err)
		}
		result, err := store.Query(context.Background(), QueryRequest{Operation: "search", Entity: "items", Query: fixture.query})
		if err != nil {
			t.Fatal(err)
		}
		rows := result.(SearchResult).Items
		if len(rows) != 1 || rows[0].(Item).ID != created.(Item).ID {
			t.Errorf("unit query %q: %#v", fixture.query, rows)
		}
	}
}
