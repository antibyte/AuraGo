package mailer

import (
	"testing"
)

func TestPOP3FallbackIDsSurviveUnrelatedRemovalAndKeepDuplicates(t *testing.T) {
	const unrelated = "From: x@example.test\r\n\r\nother"
	const duplicate = "From: x@example.test\r\n\r\nsame"
	before := make(map[string]int)
	if _, err := popRemoteID([]byte(unrelated), "", before); err != nil {
		t.Fatal(err)
	}
	first, err := popRemoteID([]byte(duplicate), "", before)
	if err != nil {
		t.Fatal(err)
	}
	second, err := popRemoteID([]byte(duplicate), "", before)
	if err != nil {
		t.Fatal(err)
	}
	afterRemoval := make(map[string]int)
	firstAfter, err := popRemoteID([]byte(duplicate), "", afterRemoval)
	if err != nil {
		t.Fatal(err)
	}
	secondAfter, err := popRemoteID([]byte(duplicate), "", afterRemoval)
	if err != nil {
		t.Fatal(err)
	}
	if first != firstAfter || second != secondAfter || first == second {
		t.Fatalf("fallback identity changed or collapsed duplicates: before=%q,%q after=%q,%q", first, second, firstAfter, secondAfter)
	}
}

func TestPOP3UIDLIdentityAndCursorBounds(t *testing.T) {
	if id, err := popRemoteID([]byte("same"), "provider-uid", nil); err != nil || id != "uidl:provider-uid" {
		t.Fatalf("UIDL identity %q: %v", id, err)
	}
	const hash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	state := pop3SyncCursor{Version: 1, Next: 50, Num: 120, FallbackCounts: map[string]int{hash: 3}}
	raw, err := encodePOPSyncCursor(state)
	if err != nil {
		t.Fatal(err)
	}
	got, legacy, err := decodePOPSyncCursor(raw)
	if err != nil || legacy || got.Next != state.Next || got.Num != state.Num || got.FallbackCounts[hash] != 3 {
		t.Fatalf("cursor round-trip mismatch: %#v legacy=%v err=%v", got, legacy, err)
	}
	if _, _, err := decodePOPSyncCursor("-1"); err == nil {
		t.Fatal("negative legacy cursor accepted")
	}
}
