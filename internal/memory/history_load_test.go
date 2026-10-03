package memory

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHistoryManagerQuarantinesCorruptFileInsteadOfOverwriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	corrupt := []byte(`{"messages":[{"role":"user","content":"precious conversation"`)
	if err := os.WriteFile(path, corrupt, 0o600); err != nil {
		t.Fatal(err)
	}

	hm := NewHistoryManager(path)
	if err := hm.Add("user", "new message", 1, false, false); err != nil {
		t.Fatalf("Add: %v", err)
	}
	hm.Close()

	matches, err := filepath.Glob(path + ".corrupt-*")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("quarantined copies = %v, want exactly one", matches)
	}
	kept, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(kept, corrupt) {
		t.Fatalf("quarantined bytes = %q, want the original file", kept)
	}
	if loadErr := hm.LoadError(); loadErr == nil || !strings.Contains(loadErr.Error(), "moved to") {
		t.Fatalf("LoadError = %v, want the quarantine to be reported", loadErr)
	}
	if hm.PersistenceBlocked() {
		t.Fatal("a quarantined history must allow a fresh history to be saved")
	}

	reloaded := NewHistoryManager(path)
	defer reloaded.Close()
	all := reloaded.GetAll()
	if len(all) != 1 || all[0].Content != "new message" {
		t.Fatalf("fresh history after quarantine = %+v, want only the new message", all)
	}
}

func TestHistoryManagerQuarantinesZeroLengthFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	hm := NewHistoryManager(path)
	defer hm.Close()
	if got := len(hm.GetAll()); got != 0 {
		t.Fatalf("messages = %d, want 0", got)
	}
	matches, err := filepath.Glob(path + ".corrupt-*")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("quarantined copies = %v, want the zero-length file moved aside", matches)
	}
}

func TestHistoryManagerReadErrorNeverOverwritesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	original := []byte(`{"messages":[{"role":"user","content":"keep me","pinned":false,"is_internal":false,"id":1}],"current_summary":"summary"}`)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	previous := readHistoryFile
	readHistoryFile = func(string) ([]byte, error) {
		return nil, &fs.PathError{Op: "open", Path: path, Err: fs.ErrPermission}
	}
	t.Cleanup(func() { readHistoryFile = previous })

	hm := NewHistoryManager(path)
	if err := hm.Add("user", "must not replace the file", 2, false, false); err != nil {
		t.Fatalf("Add: %v", err)
	}
	hm.Close()

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, original) {
		t.Fatalf("history file was overwritten after a read error: %q", after)
	}
	if !hm.PersistenceBlocked() {
		t.Fatal("PersistenceBlocked = false after a read error")
	}
	if loadErr := hm.LoadError(); loadErr == nil || !errors.Is(loadErr, fs.ErrPermission) {
		t.Fatalf("LoadError = %v, want the permission error", loadErr)
	}
}
