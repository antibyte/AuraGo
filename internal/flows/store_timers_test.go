package flows

import (
	"context"
	"testing"
	"time"
)

func storedTimers(t *testing.T, s *Store) []TimerRecord {
	t.Helper()
	list, err := s.ListTimers(context.Background())
	if err != nil {
		t.Fatalf("ListTimers: %v", err)
	}
	return list
}

func TestStoreDeleteTimerAtOnlyDeletesTheNamedOccurrence(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := createStoredFlow(t, s, "flow_aaaaaaaada")
	// The store keeps microseconds: the nanoseconds of at must not stop it from matching.
	at := storeNow.Add(time.Hour + 123456789*time.Nanosecond)
	if err := s.ReplaceTimers(ctx, f.ID, []TimerRecord{{NodeID: "n_aaaaaaab", FireAt: at}}); err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteTimerAt(ctx, f.ID, "n_aaaaaaab", at.Add(time.Hour)); err != nil {
		t.Fatalf("DeleteTimerAt for another time: %v", err)
	}
	if err := s.DeleteTimerAt(ctx, f.ID, "n_aaaaaaaz", at); err != nil {
		t.Fatalf("DeleteTimerAt for another node: %v", err)
	}
	if err := s.DeleteTimerAt(ctx, "flow_aaaaaaaaza", "n_aaaaaaab", at); err != nil {
		t.Fatalf("DeleteTimerAt for another flow: %v", err)
	}
	if list := storedTimers(t, s); len(list) != 1 {
		t.Fatalf("a timer armed for another occurrence must stay, got %+v", list)
	}

	if err := s.DeleteTimerAt(ctx, f.ID, "n_aaaaaaab", at); err != nil {
		t.Fatalf("DeleteTimerAt: %v", err)
	}
	if list := storedTimers(t, s); len(list) != 0 {
		t.Fatalf("the named occurrence must be deleted, got %+v", list)
	}
	if err := s.DeleteTimerAt(ctx, f.ID, "n_aaaaaaab", at); err != nil {
		t.Fatalf("deleting a timer that is gone is not an error: %v", err)
	}
}

func TestStoreMoveTimerOnlyMovesTheNamedOccurrence(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := createStoredFlow(t, s, "flow_aaaaaaaada")
	at := storeNow.Add(time.Hour)
	next := at.AddDate(1, 0, 0)
	if err := s.ReplaceTimers(ctx, f.ID, []TimerRecord{{NodeID: "n_aaaaaaab", FireAt: at, Repeat: RepeatYearly}}); err != nil {
		t.Fatal(err)
	}

	if err := s.MoveTimer(ctx, f.ID, "n_aaaaaaab", at.Add(time.Minute), next); err != nil {
		t.Fatalf("MoveTimer from another time: %v", err)
	}
	if list := storedTimers(t, s); len(list) != 1 || !list[0].FireAt.Equal(at) {
		t.Fatalf("a timer armed for another occurrence must not move, got %+v", list)
	}

	if err := s.MoveTimer(ctx, f.ID, "n_aaaaaaab", at, next); err != nil {
		t.Fatalf("MoveTimer: %v", err)
	}
	list := storedTimers(t, s)
	if len(list) != 1 || !list[0].FireAt.Equal(next) || list[0].Repeat != RepeatYearly {
		t.Fatalf("the named occurrence must move and keep its repeat, got %+v", list)
	}

	if err := s.MoveTimer(ctx, f.ID, "n_aaaaaaab", next, time.Time{}); err == nil {
		t.Fatal("MoveTimer must reject a zero time")
	}
	if err := s.MoveTimer(ctx, "flow_aaaaaaaaza", "n_aaaaaaab", at, next); err != nil {
		t.Fatalf("moving a timer of a missing flow is not an error: %v", err)
	}
	if list := storedTimers(t, s); len(list) != 1 || !list[0].FireAt.Equal(next) {
		t.Fatalf("timers after the rejected and missing moves = %+v", list)
	}
}
