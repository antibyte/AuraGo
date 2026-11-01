package findit

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

func TestConcurrentReservationsNeverOverbookTwelveParts(t *testing.T) {
	store := openTestStore(t)
	item := createTestItem(t, store, "Concurrent part", "CONCURRENT", "piece", false, nil)
	location := createTestLocation(t, store, "Drawer", "")
	received, err := executeTest(t, store, "stock", "", "", 0, "receive", map[string]any{"kind": "receive", "item_id": item.ID, "location_id": location.ID, "quantity": "12"})
	if err != nil {
		t.Fatal(err)
	}
	stock := received.(StockCommandResult).Stock
	project := func(name string) Project {
		created, err := executeTest(t, store, "create", "projects", "", 0, "", map[string]any{"name": name, "lines": []map[string]any{{"item_id": item.ID, "quantity": "12"}}})
		if err != nil {
			t.Fatal(err)
		}
		p := created.(Project)
		released, err := executeTest(t, store, "project_status", "projects", p.ID, p.Version, "release-"+name, map[string]any{"status": "released"})
		if err != nil {
			t.Fatal(err)
		}
		return released.(Project)
	}
	first, second, third := project("first"), project("second"), project("third")
	reserve := func(p Project, quantity, id string) error {
		body, _ := json.Marshal(map[string]any{"project_id": p.ID, "line_id": p.Lines[0].ID, "stock_id": stock.ID, "quantity": quantity})
		_, err := store.Execute(context.Background(), CommandRequest{Operation: "reserve", RequestID: id, Data: body})
		return err
	}
	if err := reserve(first, "8", "hold-eight"); err != nil {
		t.Fatal(err)
	}
	if err := reserve(second, "5", "reject-five"); ErrorCode(err) != "conflict" {
		t.Fatalf("reserve five with four free: %v", err)
	}
	assertBalance := func(onHand, reserved, available string) {
		result, err := store.Query(context.Background(), QueryRequest{Operation: "availability", Entity: "items", ID: item.ID})
		if err != nil {
			t.Fatal(err)
		}
		a := result.(AvailabilityResult)
		if a.OnHand != onHand || a.Reserved != reserved || a.Available != available {
			t.Fatalf("balance = %+v, want %s/%s/%s", a, onHand, reserved, available)
		}
	}
	assertBalance("12", "8", "4")
	var wg sync.WaitGroup
	start := make(chan struct{})
	outcomes := make(chan error, 2)
	for index, p := range []Project{second, third} {
		wg.Add(1)
		go func(p Project, index int) {
			defer wg.Done()
			<-start
			outcomes <- reserve(p, "3", fmt.Sprintf("race-%d", index))
		}(p, index)
	}
	close(start)
	wg.Wait()
	close(outcomes)
	success, conflict := 0, 0
	for err := range outcomes {
		if err == nil {
			success++
		} else if ErrorCode(err) == "conflict" {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("concurrent outcomes = %d successful, %d conflicts", success, conflict)
	}
	assertBalance("12", "11", "1")
	// Multiple simultaneous retries of an acknowledged request return its receipt.
	outcomes = make(chan error, 6)
	for range 6 {
		wg.Add(1)
		go func() { defer wg.Done(); outcomes <- reserve(first, "8", "hold-eight") }()
	}
	wg.Wait()
	close(outcomes)
	for err := range outcomes {
		if err != nil {
			t.Fatal(err)
		}
	}
	assertBalance("12", "11", "1")
}

func TestDecimalMovementAndReturnRemainExactAcrossLocations(t *testing.T) {
	store := openTestStore(t)
	item := createTestItem(t, store, "Cable spool", "CABLE-EXACT", "m", false, nil)
	from, to := createTestLocation(t, store, "Spool", ""), createTestLocation(t, store, "Bench", "")
	r, err := executeTest(t, store, "stock", "", "", 0, "decimal-receive", map[string]any{"kind": "receive", "item_id": item.ID, "location_id": from.ID, "quantity": "2.345678"})
	if err != nil {
		t.Fatal(err)
	}
	stock := r.(StockCommandResult).Stock
	_, err = executeTest(t, store, "stock", "", "", 0, "decimal-transfer", map[string]any{"kind": "transfer", "item_id": item.ID, "stock_id": stock.ID, "target_location_id": to.ID, "quantity": "1.123456"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := store.Query(context.Background(), QueryRequest{Operation: "availability", Entity: "items", ID: item.ID})
	if err != nil {
		t.Fatal(err)
	}
	a := result.(AvailabilityResult)
	if a.OnHand != "2.345678" || len(a.Stocks) != 2 {
		t.Fatalf("transfer changed total: %+v", a)
	}
	var target StockRecord
	for _, s := range a.Stocks {
		if s.LocationID == to.ID {
			target = s
		}
	}
	if target.Quantity != "1.123456" {
		t.Fatalf("target=%+v", target)
	}
	for _, op := range []string{"issue", "return"} {
		_, err = executeTest(t, store, "stock", "", "", 0, "decimal-"+op, map[string]any{"kind": op, "item_id": item.ID, "stock_id": target.ID, "quantity": "0.123456"})
		if err != nil {
			t.Fatal(err)
		}
	}
	result, err = store.Query(context.Background(), QueryRequest{Operation: "availability", Entity: "items", ID: item.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.(AvailabilityResult); got.OnHand != "2.345678" || got.Available != "2.345678" {
		t.Fatalf("return lost precision: %+v", got)
	}
}

func TestSchemaBackupPreservesDataAndNewerDatabaseIsRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "findit.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	item := createTestItem(t, store, "Preserved part", "BACKUP-PART", "piece", false, nil)
	if err := backupDatabase(store.db); err != nil {
		t.Fatal(err)
	}
	backups, err := filepath.Glob(path + ".findit-v*.bak")
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups %v, %v", backups, err)
	}
	backup, err := Open(backups[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := backup.Query(context.Background(), QueryRequest{Operation: "get", Entity: "items", ID: item.ID}); err != nil {
		t.Fatal(err)
	}
	if err := backup.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(fmt.Sprintf("PRAGMA user_version=%d", SchemaVersion+1)); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if reopened, err := Open(path); ErrorCode(err) != "unavailable" {
		if reopened != nil {
			_ = reopened.Close()
		}
		t.Fatalf("future schema accepted: %v", err)
	}
}

func TestPrintedIdentifierSearchIsExactAndBounded(t *testing.T) {
	store := openTestStore(t)
	item := createTestItem(t, store, "Label target", "LABEL-TARGET", "piece", false, nil)
	location := createTestLocation(t, store, "Label drawer", "")
	for _, tc := range []struct{ entity, id string }{{"items", item.ID}, {"locations", location.ID}} {
		result, err := store.Query(context.Background(), QueryRequest{Operation: "search", Entity: tc.entity, Query: "id:" + tc.id[:13]})
		if err != nil {
			t.Fatal(err)
		}
		if got := result.(SearchResult); got.Total != 1 {
			t.Fatalf("%s prefix search = %+v", tc.entity, got)
		}
		_, err = store.Query(context.Background(), QueryRequest{Operation: "search", Entity: tc.entity, Query: "id:%"})
		if ErrorCode(err) != "invalid" {
			t.Fatalf("%s accepted wildcard identifier: %v", tc.entity, err)
		}
	}
}

func TestEmptyStockPositionRetainsUnitAndPathForReturn(t *testing.T) {
	store := openTestStore(t)
	item := createTestItem(t, store, "Serialized scope", "SCOPE-RETURN", "piece", true, nil)
	room := createTestLocation(t, store, "Lab", "")
	shelf := createTestLocation(t, store, "Shelf", room.ID)
	result, err := executeTest(t, store, "stock", "", "", 0, "scope-receive", map[string]any{"kind": "receive", "item_id": item.ID, "location_id": shelf.ID, "serial": "SN-001", "quantity": "1"})
	if err != nil {
		t.Fatal(err)
	}
	stockID := result.(StockCommandResult).Stock.ID
	if _, err := executeTest(t, store, "stock", "", "", 0, "scope-issue", map[string]any{"kind": "issue", "item_id": item.ID, "stock_id": stockID, "quantity": "1"}); err != nil {
		t.Fatal(err)
	}
	value, err := store.Query(context.Background(), QueryRequest{Operation: "availability", Entity: "items", ID: item.ID})
	if err != nil {
		t.Fatal(err)
	}
	a := value.(AvailabilityResult)
	if a.Unit != "piece" || a.OnHand != "0" || len(a.Stocks) != 1 || a.Stocks[0].ID != stockID || len(a.Stocks[0].LocationPath) != 2 {
		t.Fatalf("empty stock lost return metadata: %+v", a)
	}
	value, err = store.Query(context.Background(), QueryRequest{Operation: "get", Entity: "items", ID: item.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(value.(Item).StockLocations) != 0 {
		t.Fatal("empty shelf advertised as holding stock")
	}
	if _, err := executeTest(t, store, "stock", "", "", 0, "scope-return", map[string]any{"kind": "return", "item_id": item.ID, "stock_id": stockID, "quantity": "1"}); err != nil {
		t.Fatal(err)
	}
}
