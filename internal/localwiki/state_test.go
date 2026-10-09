package localwiki

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestStateAndDownloadFilesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if st, err := readState(dir); st != nil || err != nil {
		t.Fatalf("missing state.json = %+v, %v", st, err)
	}
	if d, err := readDownload(dir); d != nil || err != nil {
		t.Fatalf("missing download.json = %+v, %v", d, err)
	}
	edition := Edition{Language: "de", Variant: VariantNoPic, Date: "2026-10", Name: "wikipedia_de_all_nopic_2026-10",
		FileName: "wikipedia_de_all_nopic_2026-10.zim", Size: 42, SHA256: "ab", InstalledAt: time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)}
	want := &stateFile{Edition: &edition, PendingDelete: []string{"wikipedia_de_all_nopic_2026-09.zim"}}
	if err := writeState(dir, want); err != nil {
		t.Fatal(err)
	}
	got, err := readState(dir)
	if err != nil || !reflect.DeepEqual(got, want) || got.Version != stateVersion {
		t.Fatalf("readState = %+v, %v", got, err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, stateFileName))
	if string(raw) == "" || !reflect.DeepEqual(cloneState(got), got) {
		t.Fatal("state.json not written or cloneState differs")
	}

	download := &downloadFile{Target: edition, URLs: []string{"https://a.example/x.zim"}, LastURL: "https://b.example/x.zim"}
	if err := writeDownload(dir, download); err != nil {
		t.Fatal(err)
	}
	gotDownload, err := readDownload(dir)
	if err != nil || gotDownload.Target.FileName != edition.FileName || gotDownload.LastURL != "https://b.example/x.zim" {
		t.Fatalf("readDownload = %+v, %v", gotDownload, err)
	}
	if err := removeDownload(dir); err != nil {
		t.Fatal(err)
	}
	if err := removeDownload(dir); err != nil {
		t.Fatalf("removing a missing download.json must succeed: %v", err)
	}
}

func TestStateFilesRejectUnexpectedFileNames(t *testing.T) {
	dir := t.TempDir()
	bad := Edition{FileName: "../../etc/passwd"}
	if err := writeState(dir, &stateFile{Edition: &bad}); err != nil {
		t.Fatal(err)
	}
	if _, err := readState(dir); err == nil {
		t.Fatal("state.json with a path outside the pattern was accepted")
	}
	if err := writeDownload(dir, &downloadFile{Target: bad}); err != nil {
		t.Fatal(err)
	}
	if _, err := readDownload(dir); err == nil {
		t.Fatal("download.json with a path outside the pattern was accepted")
	}
	if err := os.WriteFile(filepath.Join(dir, stateFileName), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readState(dir); err == nil {
		t.Fatal("broken state.json was accepted")
	}
}
