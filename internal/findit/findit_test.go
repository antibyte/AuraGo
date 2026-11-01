package findit

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "findit.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func executeTest(t *testing.T, store *Store, operation, entity, id string, expected int64, requestID string, data any) (any, error) {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal command data: %v", err)
	}
	return store.Execute(context.Background(), CommandRequest{
		Operation: operation, Entity: entity, ID: id, ExpectedVersion: expected, RequestID: requestID, Data: raw,
	})
}

func createTestItem(t *testing.T, store *Store, name, partNumber, unit string, serials bool, properties map[string]any) Item {
	t.Helper()
	data := map[string]any{
		"name": name, "template_id": "resistor", "manufacturer": "Yageo", "part_number": partNumber,
		"unit": unit, "track_serials": serials, "package_family": "0603", "properties": properties,
		"datasheet_urls": []string{"https://example.test/datasheet.pdf"}, "tags": []string{"resistor", "smd"},
	}
	result, err := executeTest(t, store, "create", "items", "", 0, "", data)
	if err != nil {
		t.Fatalf("create item: %v", err)
	}
	return result.(Item)
}

func createTestLocation(t *testing.T, store *Store, name, parent string) Location {
	t.Helper()
	result, err := executeTest(t, store, "create", "locations", "", 0, "", map[string]any{
		"name": name, "kind": "bin", "parent_id": parent,
	})
	if err != nil {
		t.Fatalf("create location: %v", err)
	}
	return result.(Location)
}

func TestQuantityAndEngineeringNotation(t *testing.T) {
	for _, test := range []struct {
		input string
		want  string
	}{
		{"0", "0"}, {"1.25", "1.25"}, {"0.000001", "0.000001"}, {"12.340000", "12.34"},
	} {
		value, err := ParseQuantity(test.input)
		if err != nil {
			t.Fatalf("ParseQuantity(%q): %v", test.input, err)
		}
		if got := FormatQuantity(value); got != test.want {
			t.Errorf("FormatQuantity(ParseQuantity(%q)) = %q, want %q", test.input, got, test.want)
		}
	}
	for _, input := range []string{"-1", "+1", "1e3", "1,2", "1.0000001", "9223372036855"} {
		if _, err := ParseQuantity(input); err == nil {
			t.Errorf("ParseQuantity(%q) unexpectedly succeeded", input)
		}
	}
	for input, want := range map[string]float64{"4k7": 4700, "2R2": 2.2, "1M": 1_000_000, "3.3k": 3300} {
		got, err := parseNumeric(input)
		if err != nil || got != want {
			t.Errorf("parseNumeric(%q) = %v, %v; want %v", input, got, err, want)
		}
	}
}

func TestCatalogSeedsComponentTemplatesAndPackageFamilies(t *testing.T) {
	store := openTestStore(t)
	catalog := Catalog()
	packages := make(map[string]bool, len(catalog.PackageFamilies))
	for _, family := range catalog.PackageFamilies {
		if packages[family.ID] {
			t.Fatalf("duplicate package family %q", family.ID)
		}
		packages[family.ID] = true
	}
	for _, id := range []string{"01005", "0603", "dip", "sip", "ssop", "msop", "sc70", "sod323", "to220", "to252_dpak", "to263_d2pak", "lqfp", "dfn", "lga", "plcc", "axial", "radial"} {
		if !packages[id] {
			t.Errorf("catalog missing package family %q", id)
		}
	}
	templates := make(map[string]Template, len(catalog.Templates))
	for _, template := range catalog.Templates {
		if template.IconID != template.ID {
			t.Errorf("template %q icon id = %q", template.ID, template.IconID)
		}
		templates[template.ID] = template
	}
	for _, id := range []string{"potentiometer", "led", "mosfet", "op_amp", "comparator", "voltage_regulator", "logic", "microcontroller", "memory_ic", "adc", "dac", "interface_ic", "sensor", "optoisolator", "relay", "switch", "fuse", "crystal", "oscillator", "display", "motor", "fan", "development_board", "radio_module", "single_board_computer", "expansion_card", "cpu", "motherboard", "memory", "gpu", "ssd", "hdd", "power_supply", "cpu_cooler", "pc_case", "network_card", "adapter", "meter", "battery", "fastener"} {
		if _, ok := templates[id]; !ok {
			t.Errorf("catalog missing template %q", id)
			continue
		}
		if _, err := getTemplate(context.Background(), store.db, id); err != nil {
			t.Errorf("template %q not seeded in SQLite: %v", id, err)
		}
	}
}

func TestUsedTemplateAllowsAdditiveOptionalFieldsWithoutOverwritingItems(t *testing.T) {
	store := openTestStore(t)
	created, err := executeTest(t, store, "create", "templates", "", 0, "create-template", map[string]any{
		"name": "Custom module", "category": "electronic_component", "fields": []map[string]any{{"key": "value", "label": "Value", "type": "text"}},
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	template := created.(Template)
	created, err = executeTest(t, store, "create", "items", "", 0, "create-template-item", map[string]any{
		"name": "Custom part", "template_id": template.ID, "unit": "piece", "properties": map[string]any{"value": "original"},
	})
	if err != nil {
		t.Fatalf("create template item: %v", err)
	}
	item := created.(Item)
	updated, err := executeTest(t, store, "update", "templates", template.ID, template.Version, "add-template-field", map[string]any{
		"name": "Custom module v2", "description": "Updated help", "category": "electronic_component",
		"fields": []map[string]any{
			{"key": "value", "label": "Nominal value", "type": "text"},
			{"key": "tolerance_pct", "label": "Tolerance", "type": "number", "unit": "%"},
		},
	})
	if err != nil {
		t.Fatalf("add optional template field: %v", err)
	}
	if updated.(Template).Version != template.Version+1 {
		t.Fatalf("updated template version = %d", updated.(Template).Version)
	}
	got, err := store.Query(context.Background(), QueryRequest{Operation: "get", Entity: "items", ID: item.ID})
	if err != nil {
		t.Fatal(err)
	}
	properties := got.(Item).Properties
	if properties["value"] != "original" {
		t.Fatalf("existing item property was overwritten: %#v", properties)
	}
	if _, exists := properties["tolerance_pct"]; exists {
		t.Fatalf("new optional property should remain unset on existing item: %#v", properties)
	}
	_, err = executeTest(t, store, "update", "templates", template.ID, template.Version+1, "change-template-type", map[string]any{
		"name": "Custom module v2", "description": "Updated help", "category": "electronic_component",
		"fields": []map[string]any{
			{"key": "value", "label": "Value", "type": "number"},
			{"key": "tolerance_pct", "label": "Tolerance", "type": "number", "unit": "%"},
		},
	})
	if ErrorCode(err) != "conflict" {
		t.Fatalf("incompatible used-template change error = %v, want conflict", err)
	}
}

func TestSearchTypedPropertiesAndEngineeringShorthand(t *testing.T) {
	store := openTestStore(t)
	item := createTestItem(t, store, "4.7 kOhm resistor", "RC0603-4K7", "piece", false, map[string]any{
		"resistance_ohms": json.Number("4700"), "tolerance_pct": json.Number("1"), "power_w": json.Number("0.1"),
	})
	result, err := store.Query(context.Background(), QueryRequest{Operation: "search", Entity: "items", Query: "4k7", Limit: 10})
	if err != nil {
		t.Fatalf("4k7 search: %v", err)
	}
	search := result.(SearchResult)
	if search.Total != 1 || search.Items[0].(Item).ID != item.ID {
		t.Fatalf("4k7 search result = %#v", search)
	}
	result, err = store.Query(context.Background(), QueryRequest{Operation: "search", Entity: "items", TemplateID: "resistor", Filters: []PropertyFilter{{Field: "resistance_ohms", Operator: "gte", Value: "4k7"}}})
	if err != nil {
		t.Fatalf("numeric property filter: %v", err)
	}
	if got := result.(SearchResult).Total; got != 1 {
		t.Fatalf("numeric filter total = %d, want 1", got)
	}
}

func TestDatePropertiesValidateRoundTripAndRangeFilter(t *testing.T) {
	store := openTestStore(t)
	templateResult, err := executeTest(t, store, "create", "templates", "", 0, "date-template", map[string]any{
		"name": "Calibration record", "category": "instruments", "fields": []map[string]any{
			{"key": "calibration_due", "label": "Calibration due", "type": "date"},
		},
	})
	if err != nil {
		t.Fatalf("create date template: %v", err)
	}
	template := templateResult.(Template)
	for _, invalid := range []string{"2025-02-29", "2024-13-01", "2024-02-30", "2024-2-09"} {
		_, err := executeTest(t, store, "create", "items", "", 0, "", map[string]any{
			"name": "Invalid date item", "template_id": template.ID, "unit": "piece",
			"properties": map[string]any{"calibration_due": invalid},
		})
		if ErrorDetail(err).Reason != "invalid_date" {
			t.Errorf("date %q error = %v, want invalid_date", invalid, err)
		}
	}
	created, err := executeTest(t, store, "create", "items", "", 0, "", map[string]any{
		"name": "Leap year instrument", "template_id": template.ID, "unit": "piece",
		"properties": map[string]any{"calibration_due": "2024-02-29"},
	})
	if err != nil {
		t.Fatalf("create valid date item: %v", err)
	}
	item := created.(Item)
	got, err := store.Query(context.Background(), QueryRequest{Operation: "get", Entity: "items", ID: item.ID})
	if err != nil {
		t.Fatalf("get date item: %v", err)
	}
	if gotItem := got.(Item); gotItem.Properties["calibration_due"] != "2024-02-29" {
		t.Fatalf("date property round trip = %#v", gotItem.Properties)
	}
	result, err := store.Query(context.Background(), QueryRequest{
		Operation: "search", Entity: "items", TemplateID: template.ID,
		Filters: []PropertyFilter{{Field: "calibration_due", Operator: "gte", Value: "2024-02-29"}},
	})
	if err != nil {
		t.Fatalf("date range filter: %v", err)
	}
	if search := result.(SearchResult); search.Total != 1 || search.Items[0].(Item).ID != item.ID {
		t.Fatalf("date range result = %#v", search)
	}
	if _, err := store.Query(context.Background(), QueryRequest{
		Operation: "search", Entity: "items", TemplateID: template.ID,
		Filters: []PropertyFilter{{Field: "calibration_due", Operator: "gte", Value: "2025-02-29"}},
	}); ErrorDetail(err).Reason != "invalid_date" {
		t.Fatalf("invalid date filter error = %v, want invalid_date", err)
	}
}

func TestEnumOptionsAreNormalizedBeforeStorage(t *testing.T) {
	store := openTestStore(t)
	created, err := executeTest(t, store, "create", "templates", "", 0, "enum-template", map[string]any{
		"name": "Connector", "category": "cables", "fields": []map[string]any{{
			"key": "connector_type", "label": "Connector type", "type": "enum", "options": []string{" JST "},
		}},
	})
	if err != nil {
		t.Fatalf("create enum template: %v", err)
	}
	template := created.(Template)
	if got := template.Fields[0].Options[0]; got != "JST" {
		t.Fatalf("stored enum option = %q, want trimmed JST", got)
	}
	if _, err := executeTest(t, store, "create", "items", "", 0, "", map[string]any{
		"name": "JST lead", "template_id": template.ID, "unit": "piece",
		"properties": map[string]any{"connector_type": "JST"},
	}); err != nil {
		t.Fatalf("create item with normalized enum option: %v", err)
	}
	_, err = executeTest(t, store, "create", "templates", "", 0, "", map[string]any{
		"name": "Duplicate enum", "category": "cables", "fields": []map[string]any{{
			"key": "connector_type", "label": "Connector type", "type": "enum", "options": []string{"JST", " JST "},
		}},
	})
	if ErrorCode(err) != "invalid" {
		t.Fatalf("duplicate normalized enum options error = %v, want invalid", err)
	}
}

func TestStockCommandsReservationsAndHistory(t *testing.T) {
	store := openTestStore(t)
	item := createTestItem(t, store, "Resistor", "RC0603-1K", "piece", false, map[string]any{"resistance_ohms": json.Number("1000")})
	room := createTestLocation(t, store, "Workshop", "")
	bin := createTestLocation(t, store, "Parts drawer", room.ID)
	result, err := executeTest(t, store, "stock", "", "", 0, "receive-1", map[string]any{
		"kind": "receive", "item_id": item.ID, "location_id": bin.ID, "quantity": "10", "lot": "L1",
	})
	if err != nil {
		t.Fatalf("receive stock: %v", err)
	}
	stock := result.(StockCommandResult).Stock
	if stock.Quantity != "10" || stock.Available != "10" {
		t.Fatalf("received stock = %#v", stock)
	}
	result, err = store.Query(context.Background(), QueryRequest{Operation: "search", Entity: "items", LocationID: room.ID})
	if err != nil || result.(SearchResult).Total != 1 {
		t.Fatalf("search at parent location should include nested stock: result=%#v err=%v", result, err)
	}
	if _, err := executeTest(t, store, "stock", "", "", 0, "receive-1", map[string]any{
		"kind": "receive", "item_id": item.ID, "location_id": bin.ID, "quantity": "10", "lot": "L1",
	}); err != nil {
		t.Fatalf("idempotent receive replay: %v", err)
	}
	if _, err := executeTest(t, store, "stock", "", "", 0, "receive-1", map[string]any{
		"kind": "receive", "item_id": item.ID, "location_id": bin.ID, "quantity": "11", "lot": "L1",
	}); ErrorCode(err) != "conflict" {
		t.Fatalf("request id reuse error = %v, want conflict", err)
	}

	created, err := executeTest(t, store, "create", "projects", "", 0, "", map[string]any{
		"name": "Radio build", "lines": []map[string]any{{"item_id": item.ID, "quantity": "5"}},
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	project := created.(Project)
	lineID := project.Lines[0].ID
	created, err = executeTest(t, store, "project_status", "projects", project.ID, project.Version, "release-1", map[string]any{"status": "released"})
	if err != nil {
		t.Fatalf("release project: %v", err)
	}
	project = created.(Project)
	reserved, err := executeTest(t, store, "reserve", "", "", 0, "reserve-1", map[string]any{
		"project_id": project.ID, "line_id": lineID, "stock_id": stock.ID, "quantity": "4",
	})
	if err != nil {
		t.Fatalf("reserve stock: %v", err)
	}
	reservation := reserved.(Reservation)
	if strings.Join(reservation.LocationPath, "/") != "Workshop/Parts drawer" || reservation.Remaining != "4" {
		t.Fatalf("reservation response = %#v", reservation)
	}
	if _, err := executeTest(t, store, "reserve", "", "", 0, "reserve-2", map[string]any{
		"project_id": project.ID, "line_id": lineID, "stock_id": stock.ID, "quantity": "2",
	}); ErrorCode(err) != "conflict" {
		t.Fatalf("over-reservation error = %v, want conflict", err)
	} else if detail := ErrorDetail(err); detail.Reason != "reservation_exceeds_need" || detail.Details["remaining_need"] != "1" {
		t.Fatalf("over-reservation details = %#v", detail)
	}
	other, err := executeTest(t, store, "create", "projects", "", 0, "", map[string]any{
		"name": "Other build", "lines": []map[string]any{{"item_id": item.ID, "quantity": "7"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	otherProject := other.(Project)
	if _, err := executeTest(t, store, "project_status", "projects", otherProject.ID, otherProject.Version, "release-other", map[string]any{"status": "released"}); err != nil {
		t.Fatal(err)
	}
	if _, err := executeTest(t, store, "reserve", "", "", 0, "reserve-other", map[string]any{
		"project_id": otherProject.ID, "line_id": otherProject.Lines[0].ID, "stock_id": stock.ID, "quantity": "7",
	}); ErrorCode(err) != "conflict" {
		t.Fatalf("cross-project stock over-reservation error = %v, want conflict", err)
	} else if detail := ErrorDetail(err); detail.Reason != "insufficient_available" || detail.Details["available"] != "6" {
		t.Fatalf("cross-project stock details = %#v", detail)
	}
	if _, err := executeTest(t, store, "stock", "", "", 0, "issue-blocked", map[string]any{
		"kind": "issue", "item_id": item.ID, "stock_id": stock.ID, "quantity": "7",
	}); ErrorCode(err) != "conflict" {
		t.Fatalf("issue into reservation error = %v, want conflict", err)
	} else if detail := ErrorDetail(err); detail.Reason != "insufficient_available" || detail.Details["available"] != "6" {
		t.Fatalf("issue availability details = %#v", detail)
	}
	if _, err := executeTest(t, store, "release", "", "", 0, "release-part", map[string]any{"reservation_id": reservation.ID, "quantity": "1"}); err != nil {
		t.Fatalf("release reservation: %v", err)
	}
	if _, err := executeTest(t, store, "stock", "", "", 0, "issue-available", map[string]any{
		"kind": "issue", "item_id": item.ID, "stock_id": stock.ID, "quantity": "7",
	}); err != nil {
		t.Fatalf("issue available stock: %v", err)
	}
	if _, err := executeTest(t, store, "consume", "", "", 0, "consume-held", map[string]any{"reservation_id": reservation.ID, "quantity": "3"}); err != nil {
		t.Fatalf("consume reservation: %v", err)
	}
	result, err = store.Query(context.Background(), QueryRequest{Operation: "get", Entity: "projects", ID: project.ID})
	if err != nil {
		t.Fatalf("get project detail: %v", err)
	}
	project = result.(Project)
	if len(project.Reservations) != 1 || project.Reservations[0].Consumed != "3" || project.Lines[0].Consumed != "3" {
		t.Fatalf("project reservation history = %#v", project)
	}
	result, err = store.Query(context.Background(), QueryRequest{Operation: "shortages", ID: project.ID})
	if err != nil {
		t.Fatalf("project shortages: %v", err)
	}
	shortages := result.(ShortagesResult)
	if shortages.Lines[0].Shortage != "2" || len(shortages.Reservations) != 1 {
		t.Fatalf("shortage after consume = %#v", shortages)
	}
	result, err = store.Query(context.Background(), QueryRequest{Operation: "get", Entity: "items", ID: item.ID})
	if err != nil {
		t.Fatalf("get item history: %v", err)
	}
	item = result.(Item)
	if len(item.Movements) < 4 {
		t.Fatalf("item movement history has %d entries, want at least 4", len(item.Movements))
	}
	if item.Movements[0].Quantity == "0" {
		t.Fatalf("latest movement quantity is missing: %#v", item.Movements[0])
	}
	if _, err := executeTest(t, store, "project_status", "projects", project.ID, project.Version, "complete", map[string]any{"status": "completed"}); err != nil {
		t.Fatalf("complete project after explicit consumption: %v", err)
	}
}

func TestLowStockUsesAvailableQuantityAndFilterLimit(t *testing.T) {
	store := openTestStore(t)
	created, err := executeTest(t, store, "create", "items", "", 0, "", map[string]any{
		"name": "Low stock test", "template_id": "generic", "unit": "piece", "minimum_stock": "3",
	})
	if err != nil {
		t.Fatal(err)
	}
	item := created.(Item)
	location := createTestLocation(t, store, "Drawer", "")
	received, err := executeTest(t, store, "stock", "", "", 0, "receive-low-stock", map[string]any{
		"kind": "receive", "item_id": item.ID, "location_id": location.ID, "quantity": "4",
	})
	if err != nil {
		t.Fatal(err)
	}
	stock := received.(StockCommandResult).Stock
	result, err := store.Query(context.Background(), QueryRequest{Operation: "search", Entity: "items", LowStock: true})
	if err != nil || result.(SearchResult).Total != 0 {
		t.Fatalf("item above minimum should not be low stock: result=%#v err=%v", result, err)
	}
	created, err = executeTest(t, store, "create", "projects", "", 0, "", map[string]any{
		"name": "Hold parts", "lines": []map[string]any{{"item_id": item.ID, "quantity": "2"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	project := created.(Project)
	_, err = executeTest(t, store, "project_status", "projects", project.ID, project.Version, "release-low-stock", map[string]any{"status": "released"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executeTest(t, store, "reserve", "", "", 0, "reserve-low-stock", map[string]any{
		"project_id": project.ID, "line_id": project.Lines[0].ID, "stock_id": stock.ID, "quantity": "2",
	}); err != nil {
		t.Fatal(err)
	}
	result, err = store.Query(context.Background(), QueryRequest{Operation: "search", Entity: "items", LowStock: true})
	if err != nil || result.(SearchResult).Total != 1 {
		t.Fatalf("reserved stock should lower free stock below minimum: result=%#v err=%v", result, err)
	}
	filters := make([]PropertyFilter, 9)
	if _, err := store.Query(context.Background(), QueryRequest{Operation: "search", Entity: "items", Filters: filters}); ErrorCode(err) != "invalid" {
		t.Fatalf("too many typed property filters error = %v, want invalid", err)
	}
}

func TestProjectCancellationReleasesWithoutConsuming(t *testing.T) {
	store := openTestStore(t)
	item := createTestItem(t, store, "Part", "P-1", "piece", false, nil)
	location := createTestLocation(t, store, "Shelf", "")
	stockResult, err := executeTest(t, store, "stock", "", "", 0, "receive", map[string]any{
		"kind": "receive", "item_id": item.ID, "location_id": location.ID, "quantity": "4",
	})
	if err != nil {
		t.Fatal(err)
	}
	stock := stockResult.(StockCommandResult).Stock
	created, err := executeTest(t, store, "create", "projects", "", 0, "", map[string]any{
		"name": "Cancelled build", "lines": []map[string]any{{"item_id": item.ID, "quantity": "2"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	project := created.(Project)
	_, err = executeTest(t, store, "project_status", "projects", project.ID, project.Version, "release", map[string]any{"status": "released"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = executeTest(t, store, "reserve", "", "", 0, "reserve", map[string]any{
		"project_id": project.ID, "line_id": project.Lines[0].ID, "stock_id": stock.ID, "quantity": "2",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = executeTest(t, store, "project_status", "projects", project.ID, 2, "cancel", map[string]any{"status": "cancelled"})
	if err != nil {
		t.Fatalf("cancel project: %v", err)
	}
	result, err := store.Query(context.Background(), QueryRequest{Operation: "availability", ID: item.ID})
	if err != nil {
		t.Fatal(err)
	}
	availability := result.(AvailabilityResult)
	if availability.OnHand != "4" || availability.Reserved != "0" || availability.Available != "4" {
		t.Fatalf("cancel consumed stock or retained reservation: %#v", availability)
	}
}

func TestFindItLocationCycleAndVersionChecks(t *testing.T) {
	store := openTestStore(t)
	parent := createTestLocation(t, store, "Cabinet", "")
	child := createTestLocation(t, store, "Drawer", parent.ID)
	_, err := executeTest(t, store, "update", "locations", parent.ID, parent.Version, "", map[string]any{
		"name": "Cabinet", "kind": "cabinet", "parent_id": child.ID,
	})
	if ErrorCode(err) != "invalid" {
		t.Fatalf("cycle update error = %v, want invalid", err)
	}
	_, err = executeTest(t, store, "update", "locations", parent.ID, parent.Version+1, "", map[string]any{
		"name": "Cabinet", "kind": "cabinet",
	})
	if ErrorCode(err) != "conflict" {
		t.Fatalf("stale version error = %v, want conflict", err)
	}
}

func TestCSVPreviewImportAndExport(t *testing.T) {
	store := openTestStore(t)
	csvData := "\uFEFFname;template_id;manufacturer;part_number;unit;properties\n" +
		"1k resistor;resistor;Yageo;RC-1;piece;\"{\"\"resistance_ohms\"\":1000}\"\n" +
		"duplicate;resistor;Yageo;RC-1;piece;\"{\"\"resistance_ohms\"\":1000}\"\n"
	request := ImportRequest{Entity: "items", CSV: csvData, RequestID: "import-items-1", SkipRows: []int{3}}
	preview, err := store.PreviewCSV(context.Background(), request)
	if err != nil {
		t.Fatalf("PreviewCSV: %v", err)
	}
	if preview.ValidCount != 1 || preview.InvalidCount != 0 || len(preview.Rows) != 2 || !preview.Rows[1].Skipped {
		t.Fatalf("CSV preview = %#v", preview)
	}
	result, err := store.ImportCSV(context.Background(), request)
	if err != nil {
		t.Fatalf("ImportCSV: %v", err)
	}
	imported := result.(ImportResult)
	if len(imported.Created) != 1 || len(imported.Skipped) != 1 {
		t.Fatalf("CSV import result = %#v", imported)
	}
	if _, err := store.ImportCSV(context.Background(), request); err != nil {
		t.Fatalf("idempotent CSV replay: %v", err)
	}
	if _, err := store.ImportCSV(context.Background(), ImportRequest{Entity: "items", CSV: csvData, RequestID: request.RequestID}); ErrorCode(err) != "conflict" {
		t.Fatalf("CSV request id reuse error = %v, want conflict", err)
	}
	exported, err := store.ExportCSV(context.Background(), "items", "")
	if err != nil {
		t.Fatalf("ExportCSV items: %v", err)
	}
	if !strings.Contains(string(exported), "part_number") || !strings.Contains(string(exported), "RC-1") {
		t.Fatalf("CSV export missing imported item: %s", exported)
	}
	if _, err := store.PreviewCSV(context.Background(), ImportRequest{Entity: "items", CSV: "name,name\nA,B\n"}); ErrorCode(err) != "invalid" {
		t.Fatalf("duplicate header error = %v, want invalid", err)
	}
	if _, err := store.PreviewCSV(context.Background(), ImportRequest{Entity: "items", CSV: string([]byte{0xff, ',', 'x'})}); ErrorCode(err) != "invalid" {
		t.Fatalf("invalid UTF-8 error = %v, want invalid", err)
	}
}

func TestCSVProjectLineResolutionAndAtomicValidation(t *testing.T) {
	store := openTestStore(t)
	item := createTestItem(t, store, "Capacitor", "C-100", "piece", false, nil)
	created, err := executeTest(t, store, "create", "projects", "", 0, "", map[string]any{"name": "Board", "lines": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	project := created.(Project)
	csvData := "manufacturer,part_number,quantity,note\nYageo,C-100,4,decoupling\n"
	req := ImportRequest{Entity: "project_lines", CSV: csvData, ProjectID: project.ID, ExpectedVersion: project.Version, RequestID: "import-bom-1"}
	preview, err := store.PreviewCSV(context.Background(), req)
	if err != nil {
		t.Fatalf("PreviewCSV project lines: %v", err)
	}
	if preview.ValidCount != 1 || preview.Rows[0].ResolvedItemID != item.ID {
		t.Fatalf("project line preview = %#v", preview)
	}
	badReq := req
	badReq.CSV = "manufacturer,part_number,quantity\nYageo,C-100,4\nYageo,missing,2\n"
	badReq.RequestID = "bad-bom"
	if _, err := store.ImportCSV(context.Background(), badReq); ErrorCode(err) != "invalid" {
		t.Fatalf("invalid project import error = %v, want invalid", err)
	}
	unchanged, err := store.Query(context.Background(), QueryRequest{Operation: "get", Entity: "projects", ID: project.ID})
	if err != nil || len(unchanged.(Project).Lines) != 0 {
		t.Fatalf("failed import partially changed BOM: result=%#v err=%v", unchanged, err)
	}
	result, err := store.ImportCSV(context.Background(), req)
	if err != nil {
		t.Fatalf("ImportCSV project lines: %v", err)
	}
	if got := result.(ImportResult).Version; got != project.Version+1 {
		t.Fatalf("project import version = %d, want %d", got, project.Version+1)
	}
}

func TestSerialTrackingTransferAndDuplicateSerial(t *testing.T) {
	store := openTestStore(t)
	item := createTestItem(t, store, "Serial module", "SER-1", "piece", true, nil)
	source := createTestLocation(t, store, "Source", "")
	target := createTestLocation(t, store, "Target", "")
	result, err := executeTest(t, store, "stock", "", "", 0, "serial-receive", map[string]any{
		"kind": "receive", "item_id": item.ID, "location_id": source.ID, "quantity": "1", "serial": "ABC-1",
	})
	if err != nil {
		t.Fatalf("receive serial stock: %v", err)
	}
	stock := result.(StockCommandResult).Stock
	if _, err := executeTest(t, store, "stock", "", "", 0, "serial-duplicate", map[string]any{
		"kind": "receive", "item_id": item.ID, "location_id": source.ID, "quantity": "1", "serial": "abc-1",
	}); ErrorCode(err) != "conflict" {
		t.Fatalf("duplicate serial error = %v, want conflict", err)
	}
	result, err = executeTest(t, store, "stock", "", "", 0, "serial-transfer", map[string]any{
		"kind": "transfer", "item_id": item.ID, "stock_id": stock.ID, "target_location_id": target.ID, "quantity": "1",
	})
	if err != nil {
		t.Fatalf("transfer serial stock: %v", err)
	}
	if result.(StockCommandResult).Stock.Quantity != "1" || result.(StockCommandResult).Stock.LocationID != target.ID {
		t.Fatalf("serial transfer lost/misplaced stock: %#v", result)
	}
	itemResult, err := store.Query(context.Background(), QueryRequest{Operation: "get", Entity: "items", ID: item.ID})
	if err != nil {
		t.Fatal(err)
	}
	movements := itemResult.(Item).Movements
	var transfer MovementRecord
	for _, movement := range movements {
		if movement.Kind == "transfer" {
			transfer = movement
		}
	}
	if len(movements) != 2 || strings.Join(transfer.SourceLocationPath, "/") != "Source" || strings.Join(transfer.TargetLocationPath, "/") != "Target" || transfer.Serial != "ABC-1" {
		t.Fatalf("serial movement history = %#v", movements)
	}
	if _, err := store.Execute(context.Background(), CommandRequest{Operation: "create", Entity: "items", Data: json.RawMessage(`{"name":"fraction","template_id":"generic","unit":"piece","minimum_stock":"0.5"}`)}); ErrorCode(err) != "invalid" {
		t.Fatalf("fractional piece minimum error = %v, want invalid", err)
	}
}

func TestCRUDRequestReplayIsAtomicAndVersioned(t *testing.T) {
	store := openTestStore(t)
	data := map[string]any{"name": "Repeat-safe item", "template_id": "generic", "unit": "piece"}
	first, err := executeTest(t, store, "create", "items", "", 0, "create-item", data)
	if err != nil {
		t.Fatalf("create with request id: %v", err)
	}
	created := first.(Item)
	second, err := executeTest(t, store, "create", "items", "", 0, "create-item", data)
	if err != nil {
		t.Fatalf("replay create: %v", err)
	}
	var replayed Item
	if raw, ok := second.(json.RawMessage); !ok || json.Unmarshal(raw, &replayed) != nil || replayed.ID != created.ID {
		t.Fatalf("create replay = %#v, want original item %q", second, created.ID)
	}
	if _, err := executeTest(t, store, "create", "items", "", 0, "create-item", map[string]any{"name": "Different"}); ErrorCode(err) != "conflict" {
		t.Fatalf("reused create request id error = %v, want conflict", err)
	}
	updateData := map[string]any{
		"name": "Repeat-safe item updated", "template_id": "generic", "category": "other", "unit": "piece",
		"minimum_stock": "0", "tags": []string{}, "datasheet_urls": []string{}, "properties": map[string]any{},
	}
	updated, err := executeTest(t, store, "update", "items", created.ID, 1, "update-item", updateData)
	if err != nil {
		t.Fatalf("update with request id: %v", err)
	}
	updatedItem := updated.(Item)
	updateReplay, err := executeTest(t, store, "update", "items", created.ID, 1, "update-item", updateData)
	if err != nil {
		t.Fatalf("replay update: %v", err)
	}
	var replayedUpdate Item
	if raw, ok := updateReplay.(json.RawMessage); !ok || json.Unmarshal(raw, &replayedUpdate) != nil || replayedUpdate.Version != updatedItem.Version {
		t.Fatalf("update replay = %#v", updateReplay)
	}
	archived, err := executeTest(t, store, "archive", "items", created.ID, updatedItem.Version, "archive-item", map[string]any{})
	if err != nil {
		t.Fatalf("archive with request id: %v", err)
	}
	archiveReplay, err := executeTest(t, store, "archive", "items", created.ID, updatedItem.Version, "archive-item", map[string]any{})
	if err != nil {
		t.Fatalf("replay archive: %v", err)
	}
	var replayedArchive map[string]any
	if raw, ok := archiveReplay.(json.RawMessage); !ok || json.Unmarshal(raw, &replayedArchive) != nil || replayedArchive["archived"] != true {
		t.Fatalf("archive replay = %#v (initial %#v)", archiveReplay, archived)
	}
}

func TestProjectReservationHistoryKeepsActiveRowsVisible(t *testing.T) {
	store := openTestStore(t)
	item := createTestItem(t, store, "Header", "HDR-1", "piece", false, nil)
	location := createTestLocation(t, store, "Box", "")
	stockResult, err := executeTest(t, store, "stock", "", "", 0, "receive-many", map[string]any{
		"kind": "receive", "item_id": item.ID, "location_id": location.ID, "quantity": "2",
	})
	if err != nil {
		t.Fatal(err)
	}
	stock := stockResult.(StockCommandResult).Stock
	created, err := executeTest(t, store, "create", "projects", "", 0, "", map[string]any{
		"name": "Reservation history", "lines": []map[string]any{{"item_id": item.ID, "quantity": "2"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	project := created.(Project)
	_, err = executeTest(t, store, "project_status", "projects", project.ID, project.Version, "release-history", map[string]any{"status": "released"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 101; i++ {
		reservationID := fmt.Sprintf("reserve-history-%03d", i)
		reservation, err := executeTest(t, store, "reserve", "", "", 0, reservationID, map[string]any{
			"project_id": project.ID, "line_id": project.Lines[0].ID, "stock_id": stock.ID, "quantity": "1",
		})
		if err != nil {
			t.Fatalf("reserve history row %d: %v", i, err)
		}
		if _, err := executeTest(t, store, "release", "", "", 0, "release-"+reservationID, map[string]any{"reservation_id": reservation.(Reservation).ID, "quantity": "1"}); err != nil {
			t.Fatalf("release history row %d: %v", i, err)
		}
	}
	active, err := executeTest(t, store, "reserve", "", "", 0, "reserve-active", map[string]any{
		"project_id": project.ID, "line_id": project.Lines[0].ID, "stock_id": stock.ID, "quantity": "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := store.Query(context.Background(), QueryRequest{Operation: "get", Entity: "projects", ID: project.ID})
	if err != nil {
		t.Fatal(err)
	}
	project = result.(Project)
	if !project.ReservationsHasMore || len(project.Reservations) != 101 || project.Reservations[0].ID != active.(Reservation).ID || project.Reservations[0].Remaining != "1" {
		t.Fatalf("project reservation page omitted an active hold: hasMore=%v count=%d first=%#v", project.ReservationsHasMore, len(project.Reservations), project.Reservations[0])
	}
}

func BenchmarkSearchItems50k(b *testing.B) {
	store, err := Open(filepath.Join(b.TempDir(), "findit-benchmark.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()
	tx, err := store.db.BeginTx(context.Background(), nil)
	if err != nil {
		b.Fatal(err)
	}
	itemStmt, err := tx.Prepare(`INSERT INTO items(id,version,name,category,template_id,unit,minimum_stock_micro,tags_json,datasheet_urls_json,properties_json,created_at,updated_at)
		VALUES(?,1,?,'other','generic','piece',0,'[]','[]','{}','2026-01-01','2026-01-01')`)
	if err != nil {
		b.Fatal(err)
	}
	ftsStmt, err := tx.Prepare(`INSERT INTO findit_items_fts(item_id,name,description,manufacturer,part_number,category,package_family,tags,properties) VALUES(?,?,?,'','','other','','fixture','')`)
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 50_000; i++ {
		id := fmt.Sprintf("fixture-%05d", i)
		name := fmt.Sprintf("Fixture component %05d", i)
		if _, err := itemStmt.Exec(id, name); err != nil {
			b.Fatal(err)
		}
		if _, err := ftsStmt.Exec(id, name, "searchable 50k fixture"); err != nil {
			b.Fatal(err)
		}
	}
	_ = itemStmt.Close()
	_ = ftsStmt.Close()
	if err := tx.Commit(); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.Query(context.Background(), QueryRequest{Operation: "search", Entity: "items", Query: "searchable", Limit: 50}); err != nil {
			b.Fatal(err)
		}
	}
}
