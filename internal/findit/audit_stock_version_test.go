package findit

import (
	"context"
	"encoding/json"
	"testing"
)

func TestStockVersionPreconditionRejectsStaleMutations(t *testing.T) {
	for _, kind := range []string{"receive", "issue", "adjust", "return", "transfer"} {
		t.Run(kind, func(t *testing.T) {
			store := openTestStore(t)
			item := createTestItem(t, store, "Versioned stock "+kind, "STOCK-VERSION-"+kind, "m", false, nil)
			location := createTestLocation(t, store, "Version source "+kind, "")
			target := createTestLocation(t, store, "Version target "+kind, "")
			created, err := executeTest(t, store, "stock", "", "", 0, "version-seed-"+kind, map[string]any{
				"kind": "receive", "item_id": item.ID, "location_id": location.ID, "quantity": "10",
			})
			if err != nil {
				t.Fatalf("seed stock: %v", err)
			}
			stock := created.(StockCommandResult).Stock

			var intervening map[string]any
			switch kind {
			case "receive", "adjust":
				intervening = map[string]any{"kind": "receive", "item_id": item.ID, "stock_id": stock.ID, "location_id": location.ID, "quantity": "1"}
			case "issue", "return", "transfer":
				intervening = map[string]any{"kind": "adjust", "item_id": item.ID, "stock_id": stock.ID, "quantity": "9"}
			}
			if kind == "return" {
				intervening = map[string]any{"kind": "issue", "item_id": item.ID, "stock_id": stock.ID, "quantity": "1"}
			}
			if _, err := executeTest(t, store, "stock", "", "", 0, "version-intervening-"+kind, intervening); err != nil {
				t.Fatalf("intervening %s: %v", intervening["kind"], err)
			}

			stale := map[string]any{"kind": kind, "item_id": item.ID, "stock_id": stock.ID, "quantity": "1", "stock_version": stock.Version}
			switch kind {
			case "receive":
				stale["location_id"] = location.ID
			case "adjust":
				stale["quantity"] = "2"
			case "transfer":
				stale["target_location_id"] = target.ID
			}
			if _, err := executeTest(t, store, "stock", "", "", 0, "version-stale-"+kind, stale); ErrorCode(err) != "conflict" {
				t.Fatalf("stale %s error = %v, want conflict", kind, err)
			}
		})
	}
}

func TestStockVersionPreconditionAllowsFreshWriteAndReplaysReceipt(t *testing.T) {
	store := openTestStore(t)
	item := createTestItem(t, store, "Versioned adjust stock", "STOCK-VERSION-REPLAY", "m", false, nil)
	location := createTestLocation(t, store, "Version replay shelf", "")
	created, err := executeTest(t, store, "stock", "", "", 0, "version-replay-seed", map[string]any{
		"kind": "receive", "item_id": item.ID, "location_id": location.ID, "quantity": "10",
	})
	if err != nil {
		t.Fatal(err)
	}
	stock := created.(StockCommandResult).Stock
	request := map[string]any{"kind": "adjust", "item_id": item.ID, "stock_id": stock.ID, "quantity": "12", "stock_version": stock.Version}
	first, err := executeTest(t, store, "stock", "", "", 0, "version-replay-adjust", request)
	if err != nil {
		t.Fatalf("fresh versioned adjust: %v", err)
	}
	firstResult := first.(StockCommandResult)
	if firstResult.Stock.Quantity != "12" || firstResult.Stock.Version != stock.Version+1 {
		t.Fatalf("fresh versioned adjust result = %+v", firstResult)
	}
	// Receipt lookup precedes execution and version validation, so the exact
	// retry remains successful even though its stock_version is now stale.
	replayed, err := executeTest(t, store, "stock", "", "", 0, "version-replay-adjust", request)
	if err != nil {
		t.Fatalf("replay versioned adjust: %v", err)
	}
	var replayedResult StockCommandResult
	if raw, ok := replayed.(json.RawMessage); ok {
		if decodeErr := json.Unmarshal(raw, &replayedResult); decodeErr != nil {
			t.Fatalf("decode replayed receipt: %v", decodeErr)
		}
	} else if value, ok := replayed.(StockCommandResult); ok {
		replayedResult = value
	} else {
		t.Fatalf("replayed receipt type = %T", replayed)
	}
	if replayedResult.Stock.Quantity != "12" {
		t.Fatalf("stale version receipt replay = %#v, %v", replayedResult, err)
	}
	if _, err := executeTest(t, store, "stock", "", "", 0, "version-replay-new-adjust", map[string]any{
		"kind": "adjust", "item_id": item.ID, "stock_id": stock.ID, "quantity": "11", "stock_version": stock.Version,
	}); ErrorCode(err) != "conflict" {
		t.Fatalf("new stale versioned adjust error = %v, want conflict", err)
	}
	availability, err := store.Query(context.Background(), QueryRequest{Operation: "availability", Entity: "items", ID: item.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got := availability.(AvailabilityResult).OnHand; got != "12" {
		t.Fatalf("on hand after stale adjustment = %q, want 12", got)
	}
}

func TestStockVersionRequiresExistingPosition(t *testing.T) {
	store := openTestStore(t)
	item := createTestItem(t, store, "Unversioned receive", "STOCK-VERSION-NONE", "m", false, nil)
	location := createTestLocation(t, store, "Versionless shelf", "")
	_, err := executeTest(t, store, "stock", "", "", 0, "version-without-stock", map[string]any{
		"kind": "receive", "item_id": item.ID, "location_id": location.ID, "quantity": "1", "stock_version": 1,
	})
	if ErrorCode(err) != "invalid" {
		t.Fatalf("stock_version without stock_id error = %v, want invalid", err)
	}
}
