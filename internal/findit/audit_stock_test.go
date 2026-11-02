package findit

import (
	"context"
	"testing"
)

func TestArchivedProjectCannotChangeLifecycle(t *testing.T) {
	store := openTestStore(t)
	item := createTestItem(t, store, "Archived project part", "ARCHIVED-PROJECT", "piece", false, nil)
	created, err := executeTest(t, store, "create", "projects", "", 0, "", map[string]any{
		"name": "Archived draft", "lines": []map[string]any{{"item_id": item.ID, "quantity": "1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	project := created.(Project)
	if _, err := executeTest(t, store, "archive", "projects", project.ID, project.Version, "archive-draft", nil); err != nil {
		t.Fatalf("archive draft project: %v", err)
	}

	_, err = executeTest(t, store, "project_status", "projects", project.ID, project.Version+1, "release-archived-project", map[string]any{"status": "released"})
	if ErrorCode(err) != "not_found" {
		t.Fatalf("change archived project status error = %v, want not_found", err)
	}
	result, err := store.Query(context.Background(), QueryRequest{Operation: "get", Entity: "projects", ID: project.ID})
	if err != nil {
		t.Fatal(err)
	}
	got := result.(Project)
	if !got.Archived || got.Status != "draft" || got.Version != project.Version+1 {
		t.Fatalf("archived project changed after rejected lifecycle command: %+v", got)
	}
	var releases int
	if err := store.db.QueryRow(`SELECT count(*) FROM project_releases WHERE project_id=?`, project.ID).Scan(&releases); err != nil {
		t.Fatal(err)
	}
	if releases != 0 {
		t.Fatalf("archived project has %d release snapshots, want 0", releases)
	}
}

func TestArchivedLocationCannotReceiveStockAgain(t *testing.T) {
	for _, operation := range []string{"return", "adjust"} {
		t.Run(operation, func(t *testing.T) {
			store := openTestStore(t)
			item := createTestItem(t, store, "Archived location part", "ARCHIVED-LOCATION-"+operation, "m", false, nil)
			location := createTestLocation(t, store, "Retired shelf", "")
			received, err := executeTest(t, store, "stock", "", "", 0, "receive-"+operation, map[string]any{
				"kind": "receive", "item_id": item.ID, "location_id": location.ID, "quantity": "1",
			})
			if err != nil {
				t.Fatalf("receive fixture stock: %v", err)
			}
			stock := received.(StockCommandResult).Stock
			if _, err := executeTest(t, store, "stock", "", "", 0, "issue-"+operation, map[string]any{
				"kind": "issue", "item_id": item.ID, "stock_id": stock.ID, "quantity": "1",
			}); err != nil {
				t.Fatalf("issue fixture stock: %v", err)
			}
			if _, err := executeTest(t, store, "archive", "locations", location.ID, location.Version, "archive-location-"+operation, nil); err != nil {
				t.Fatalf("archive empty location: %v", err)
			}

			data := map[string]any{"kind": operation, "item_id": item.ID, "stock_id": stock.ID, "quantity": "1"}
			if _, err := executeTest(t, store, "stock", "", "", 0, "repopulate-"+operation, data); ErrorCode(err) != "not_found" {
				t.Fatalf("%s into archived location error = %v, want not_found", operation, err)
			}
			availability, err := store.Query(context.Background(), QueryRequest{Operation: "availability", Entity: "items", ID: item.ID})
			if err != nil {
				t.Fatal(err)
			}
			if got := availability.(AvailabilityResult); got.OnHand != "0" {
				t.Fatalf("archived location regained stock: %+v", got)
			}
		})
	}
}

func TestIssuedSerialMustBeRestoredWithReturn(t *testing.T) {
	store := openTestStore(t)
	item := createTestItem(t, store, "Returned serial module", "SERIAL-RETURN-ONLY", "piece", true, nil)
	location := createTestLocation(t, store, "Serial shelf", "")
	received, err := executeTest(t, store, "stock", "", "", 0, "serial-receive-before-issue", map[string]any{
		"kind": "receive", "item_id": item.ID, "location_id": location.ID, "quantity": "1", "serial": "SN-RETURN-ONLY",
	})
	if err != nil {
		t.Fatal(err)
	}
	stock := received.(StockCommandResult).Stock
	if _, err := executeTest(t, store, "stock", "", "", 0, "serial-issue-before-return", map[string]any{
		"kind": "issue", "item_id": item.ID, "stock_id": stock.ID, "quantity": "1",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := executeTest(t, store, "stock", "", "", 0, "serial-receive-after-issue", map[string]any{
		"kind": "receive", "item_id": item.ID, "stock_id": stock.ID, "location_id": location.ID, "quantity": "1", "serial": "SN-RETURN-ONLY",
	}); ErrorCode(err) != "conflict" {
		t.Fatalf("receive of an issued serial error = %v, want conflict", err)
	}
	returned, err := executeTest(t, store, "stock", "", "", 0, "serial-return-after-issue", map[string]any{
		"kind": "return", "item_id": item.ID, "stock_id": stock.ID, "quantity": "1",
	})
	if err != nil {
		t.Fatalf("return issued serial: %v", err)
	}
	if got := returned.(StockCommandResult); got.Kind != "return" || got.Stock.Quantity != "1" {
		t.Fatalf("serial return result = %+v", got)
	}
}

func TestItemStockTotalCannotOverflowAndTransferPreservesFullRange(t *testing.T) {
	t.Run("receive", func(t *testing.T) {
		store := openTestStore(t)
		item := createTestItem(t, store, "Large cable quantity", "MAX-RECEIVE", "m", false, nil)
		first, second := createTestLocation(t, store, "Bulk shelf", ""), createTestLocation(t, store, "Overflow shelf", "")
		max := "9223372036854.775807"
		received, err := executeTest(t, store, "stock", "", "", 0, "receive-max-stock", map[string]any{
			"kind": "receive", "item_id": item.ID, "location_id": first.ID, "quantity": max,
		})
		if err != nil {
			t.Fatalf("receive maximum stock: %v", err)
		}
		stock := received.(StockCommandResult).Stock
		if _, err := executeTest(t, store, "stock", "", "", 0, "receive-overflow-stock", map[string]any{
			"kind": "receive", "item_id": item.ID, "location_id": second.ID, "quantity": "0.000001",
		}); ErrorCode(err) != "conflict" {
			t.Fatalf("receive beyond item total range error = %v, want conflict", err)
		}
		if _, err := executeTest(t, store, "stock", "", "", 0, "transfer-max-stock", map[string]any{
			"kind": "transfer", "item_id": item.ID, "stock_id": stock.ID, "target_location_id": second.ID, "quantity": "0.000001",
		}); err != nil {
			t.Fatalf("transfer within full item quantity range: %v", err)
		}
		availability, err := store.Query(context.Background(), QueryRequest{Operation: "availability", Entity: "items", ID: item.ID})
		if err != nil {
			t.Fatalf("read maximum item availability: %v", err)
		}
		if got := availability.(AvailabilityResult); got.OnHand != max {
			t.Fatalf("item total after transfer = %q, want %q", got.OnHand, max)
		}
	})

	t.Run("return", func(t *testing.T) {
		store := openTestStore(t)
		item := createTestItem(t, store, "Large returned cable", "MAX-RETURN", "m", false, nil)
		location := createTestLocation(t, store, "Return shelf", "")
		maxMinusOne := "9223372036854.775806"
		if _, err := executeTest(t, store, "stock", "", "", 0, "receive-max-return-base", map[string]any{
			"kind": "receive", "item_id": item.ID, "location_id": location.ID, "quantity": maxMinusOne,
		}); err != nil {
			t.Fatalf("receive maximum stock: %v", err)
		}
		second, err := executeTest(t, store, "stock", "", "", 0, "receive-return-position", map[string]any{
			"kind": "receive", "item_id": item.ID, "location_id": location.ID, "quantity": "0.000001", "lot": "returned-unit",
		})
		if err != nil {
			t.Fatalf("receive return position: %v", err)
		}
		stock := second.(StockCommandResult).Stock
		if _, err := executeTest(t, store, "stock", "", "", 0, "issue-return-position", map[string]any{
			"kind": "issue", "item_id": item.ID, "stock_id": stock.ID, "quantity": "0.000001",
		}); err != nil {
			t.Fatalf("issue return position: %v", err)
		}
		if _, err := executeTest(t, store, "stock", "", "", 0, "return-overflow-stock", map[string]any{
			"kind": "return", "item_id": item.ID, "stock_id": stock.ID, "quantity": "0.000002",
		}); ErrorCode(err) != "conflict" {
			t.Fatalf("return beyond item total range error = %v, want conflict", err)
		}
	})

	t.Run("adjust", func(t *testing.T) {
		store := openTestStore(t)
		item := createTestItem(t, store, "Large adjusted cable", "MAX-ADJUST", "m", false, nil)
		first, second := createTestLocation(t, store, "Adjust shelf", ""), createTestLocation(t, store, "Adjust overflow shelf", "")
		firstQuantity := "9223372036854.775806"
		firstResult, err := executeTest(t, store, "stock", "", "", 0, "receive-adjust-first", map[string]any{
			"kind": "receive", "item_id": item.ID, "location_id": first.ID, "quantity": firstQuantity,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := executeTest(t, store, "stock", "", "", 0, "receive-adjust-second", map[string]any{
			"kind": "receive", "item_id": item.ID, "location_id": second.ID, "quantity": "0.000001",
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := executeTest(t, store, "stock", "", "", 0, "adjust-overflow-stock", map[string]any{
			"kind": "adjust", "item_id": item.ID, "stock_id": firstResult.(StockCommandResult).Stock.ID, "quantity": "9223372036854.775807",
		}); ErrorCode(err) != "conflict" {
			t.Fatalf("adjust beyond item total range error = %v, want conflict", err)
		}
	})
}

func TestProjectLifecycleHandlesLargeReservationsWithoutAggregateOverflow(t *testing.T) {
	store := openTestStore(t)
	location := createTestLocation(t, store, "Large project stock", "")
	max := "9223372036854.775807"
	items := []Item{
		createTestItem(t, store, "Large project part A", "PROJECT-MAX-A", "m", false, nil),
		createTestItem(t, store, "Large project part B", "PROJECT-MAX-B", "m", false, nil),
	}
	stocks := make(map[string]string, len(items))
	for _, item := range items {
		received, err := executeTest(t, store, "stock", "", "", 0, "receive-project-max-"+item.PartNumber, map[string]any{
			"kind": "receive", "item_id": item.ID, "location_id": location.ID, "quantity": max,
		})
		if err != nil {
			t.Fatalf("receive stock for item %s: %v", item.ID, err)
		}
		stocks[item.ID] = received.(StockCommandResult).Stock.ID
	}
	created, err := executeTest(t, store, "create", "projects", "", 0, "", map[string]any{
		"name": "Maximum independent reservations", "lines": []map[string]any{
			{"item_id": items[0].ID, "quantity": max},
			{"item_id": items[1].ID, "quantity": max},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	project := created.(Project)
	result, err := executeTest(t, store, "project_status", "projects", project.ID, project.Version, "release-project-max", map[string]any{"status": "released"})
	if err != nil {
		t.Fatal(err)
	}
	project = result.(Project)
	for _, line := range project.Lines {
		if _, err := executeTest(t, store, "reserve", "", "", 0, "reserve-project-max-"+line.ItemID, map[string]any{
			"project_id": project.ID, "line_id": line.ID, "stock_id": stocks[line.ItemID], "quantity": max,
		}); err != nil {
			t.Fatalf("reserve maximum quantity for item %s: %v", line.ItemID, err)
		}
	}

	for _, test := range []struct {
		name      string
		operation string
		entity    string
		data      any
	}{
		{name: "complete", operation: "project_status", entity: "projects", data: map[string]any{"status": "completed"}},
		{name: "reopen", operation: "project_status", entity: "projects", data: map[string]any{"status": "draft"}},
		{name: "archive", operation: "archive", entity: "projects", data: nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := executeTest(t, store, test.operation, test.entity, project.ID, project.Version, "large-project-"+test.name, test.data); ErrorCode(err) != "conflict" {
				t.Fatalf("%s project with active reservations error = %v, want conflict", test.name, err)
			}
		})
	}
}
