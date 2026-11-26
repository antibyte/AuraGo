package findit

import (
	"database/sql"
	"strings"
	"testing"
)

func stockCommand(t *testing.T, store *Store, requestID string, data map[string]any) StockCommandResult {
	t.Helper()
	result, err := executeTest(t, store, "stock", "", "", 0, requestID, data)
	if err != nil {
		t.Fatalf("%s: %v", requestID, err)
	}
	return result.(StockCommandResult)
}

func TestReceiveJoinsMatchingPositionAndReportsPath(t *testing.T) {
	store := openTestStore(t)
	item := createTestItem(t, store, "Resistor 10k", "R-10K-LEDGER", "piece", false, nil)
	cabinet := createTestLocation(t, store, "Cabinet", "")
	drawer := createTestLocation(t, store, "Drawer 3", cabinet.ID)

	first := stockCommand(t, store, "receive-1", map[string]any{"kind": "receive", "item_id": item.ID, "location_id": drawer.ID, "quantity": "10"})
	second := stockCommand(t, store, "receive-2", map[string]any{"kind": "receive", "item_id": item.ID, "location_id": drawer.ID, "quantity": "5"})
	if second.Stock.ID != first.Stock.ID || second.Stock.Quantity != "15" {
		t.Fatalf("second receive = %+v, want the first position with 15", second.Stock)
	}
	if got := strings.Join(second.Stock.LocationPath, "/"); got != "Cabinet/Drawer 3" {
		t.Fatalf("stock result location_path = %q, want Cabinet/Drawer 3", got)
	}
	other := stockCommand(t, store, "receive-used", map[string]any{"kind": "receive", "item_id": item.ID, "location_id": drawer.ID, "quantity": "1", "condition": "used"})
	if other.Stock.ID == first.Stock.ID {
		t.Fatal("receive with a different condition joined the new-condition position")
	}
	var positions int
	if err := store.db.QueryRow(`SELECT count(*) FROM stocks WHERE item_id=?`, item.ID).Scan(&positions); err != nil {
		t.Fatal(err)
	}
	if positions != 2 {
		t.Fatalf("positions = %d, want 2", positions)
	}
}

func TestTransferAndAdjustLedgerRecordsStockAttributes(t *testing.T) {
	store := openTestStore(t)
	item := createTestItem(t, store, "Cable", "CABLE-LEDGER", "m", false, nil)
	from := createTestLocation(t, store, "Shelf A", "")
	to := createTestLocation(t, store, "Shelf B", "")
	received := stockCommand(t, store, "receive", map[string]any{"kind": "receive", "item_id": item.ID, "location_id": from.ID, "quantity": "4", "lot": "L-7", "condition": "used"})
	stockCommand(t, store, "transfer", map[string]any{"kind": "transfer", "item_id": item.ID, "stock_id": received.Stock.ID, "target_location_id": to.ID, "quantity": "1"})
	var lot, condition string
	if err := store.db.QueryRow(`SELECT lot,condition FROM stock_movements WHERE request_id='transfer'`).Scan(&lot, &condition); err != nil {
		t.Fatal(err)
	}
	if lot != "L-7" || condition != "used" {
		t.Fatalf("transfer ledger lot/condition = %q/%q, want L-7/used", lot, condition)
	}

	stockCommand(t, store, "adjust-up", map[string]any{"kind": "adjust", "item_id": item.ID, "stock_id": received.Stock.ID, "quantity": "5"})
	var source, target sql.NullString
	if err := store.db.QueryRow(`SELECT source_location_id,target_location_id FROM stock_movements WHERE request_id='adjust-up'`).Scan(&source, &target); err != nil {
		t.Fatal(err)
	}
	if source.Valid || target.String != from.ID {
		t.Fatalf("positive adjustment source/target = %v/%v, want none/%s", source, target, from.ID)
	}
}

func TestIssuedSerialReturnsToActiveLocationAfterArchive(t *testing.T) {
	store := openTestStore(t)
	item := createTestItem(t, store, "Scope probe", "PROBE-SERIAL", "piece", true, nil)
	old := createTestLocation(t, store, "Old bench", "")
	next := createTestLocation(t, store, "New bench", "")
	received := stockCommand(t, store, "receive", map[string]any{"kind": "receive", "item_id": item.ID, "location_id": old.ID, "quantity": "1", "serial": "SN-1"})
	stockCommand(t, store, "issue", map[string]any{"kind": "issue", "item_id": item.ID, "stock_id": received.Stock.ID, "quantity": "1"})
	if _, err := executeTest(t, store, "archive", "locations", old.ID, old.Version, "archive-old", nil); err != nil {
		t.Fatalf("archive location with issued serial: %v", err)
	}
	returned := stockCommand(t, store, "return", map[string]any{"kind": "return", "item_id": item.ID, "stock_id": received.Stock.ID, "location_id": next.ID, "quantity": "1"})
	if returned.Stock.LocationID != next.ID || returned.Stock.Quantity != "1" {
		t.Fatalf("returned serial = %+v, want one piece at the new bench", returned.Stock)
	}
}
