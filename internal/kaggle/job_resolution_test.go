package kaggle

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"aurago/internal/config"
)

func TestKaggleUnknownResolutionPreservesReceiptAndSlotAcrossRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "kaggle.db")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if store != nil {
			store.Close()
		}
	}()
	request, _ := json.Marshal(Request{Domain: "notebooks", Operation: "save_kernel", Parameters: json.RawMessage(`{"kernelExecutionType":"SAVE_AND_RUN_ALL"}`)})
	original := Job{ID: newID(), Account: "owner", Domain: "notebooks", Operation: "save_kernel", Hash: "request-hash", Request: request}
	if _, _, err = store.admit(ctx, original); err != nil {
		t.Fatal(err)
	}
	if err = store.update(ctx, original.ID, "unknown", 42, json.RawMessage(`{"ref":"owner/book"}`), "response lost"); err != nil {
		t.Fatal(err)
	}
	before, _ := store.Get(ctx, original.ID)
	// Admission counts the newly queued job together with retained compute receipts.
	queued := original
	queued.ID = newID()
	if _, _, err = store.admit(ctx, queued); err != nil {
		t.Fatal(err)
	}
	s := &Service{Store: store}
	if s.checkComputeSlots(ctx, config.DefaultKaggleConfig()) == nil {
		t.Fatal("unknown receipt did not retain the slot")
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := store.ResolveUnknown(ctx, original.ID); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	after, err := store.Get(ctx, original.ID)
	if err != nil || after.Status != "resolved_unknown" || after.Updated == before.Updated {
		t.Fatalf("resolution not persisted: %+v %v", after, err)
	}
	preserved := after
	preserved.Status, preserved.Updated = before.Status, before.Updated
	if !reflect.DeepEqual(preserved, before) {
		t.Fatal("resolution changed the original receipt")
	}
	if err = s.checkComputeSlots(ctx, config.DefaultKaggleConfig()); err != nil {
		t.Fatal("slot not released", err)
	}
	extra := queued
	extra.ID = newID()
	if _, _, err = store.admit(ctx, extra); err != nil {
		t.Fatal(err)
	}
	if s.checkComputeSlots(ctx, config.DefaultKaggleConfig()) == nil {
		t.Fatal("resolution released more than one slot")
	}
	events, err := store.Events(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	resolved := 0
	for _, event := range events {
		var job Job
		if json.Unmarshal(event.Data, &job) == nil && job.Status == "resolved_unknown" {
			resolved++
		}
	}
	if resolved != 1 {
		t.Fatalf("got %d resolution events", resolved)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	replay, fresh, err := store.admit(ctx, original)
	if err != nil || fresh || !reflect.DeepEqual(replay, after) {
		t.Fatal("restart replay changed or repeated the receipt", err)
	}
	repeated, err := store.ResolveUnknown(ctx, original.ID)
	if err != nil || !reflect.DeepEqual(repeated, after) {
		t.Fatal("repeat was not idempotent", err)
	}
	original.Account = "other"
	if _, _, err = store.admit(ctx, original); err == nil {
		t.Fatal("account binding was lost")
	}
}

func TestKaggleUnknownResolutionRejectsOtherStates(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	for _, state := range []string{"queued", "running", "remote_running", "cancel_requested", "completed", "failed", "cancelled"} {
		id := newID()
		s.Store.admit(ctx, Job{ID: id, Account: "owner", Hash: "hash", Request: json.RawMessage(`{}`)})
		s.Store.update(ctx, id, state, 0, nil, "")
		if _, err := s.Store.ResolveUnknown(ctx, id); !errors.Is(err, ErrJobStateConflict) {
			t.Errorf("%s: %v", state, err)
		}
	}
	if _, err := s.Store.ResolveUnknown(ctx, newID()); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("missing job", err)
	}
	if _, err := s.Store.ResolveUnknown(ctx, "../invalid"); err == nil {
		t.Fatal("invalid job ID admitted")
	}
}
