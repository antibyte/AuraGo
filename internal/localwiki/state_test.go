package localwiki

import (
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

const stateTestFile = "wikipedia_de_all_nopic_2026-10.zim"

func stateTestEdition() Edition {
	return Edition{Language: "de", Variant: VariantNoPic, Date: "2026-10", Name: "wikipedia_de_all_nopic_2026-10",
		FileName: stateTestFile, Size: 42, SHA256: strings.Repeat("ab", 32), InstalledAt: time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)}
}

func TestStateAndDownloadFilesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if st, err := readState(dir); st != nil || err != nil {
		t.Fatalf("missing state.json = %+v, %v", st, err)
	}
	if d, err := readDownload(dir); d != nil || err != nil {
		t.Fatalf("missing download.json = %+v, %v", d, err)
	}
	edition := stateTestEdition()
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

	download := &downloadFile{Target: edition, URLs: []string{"https://a.example/x/" + stateTestFile}, LastURL: "https://b.example/" + stateTestFile}
	if err := writeDownload(dir, download); err != nil {
		t.Fatal(err)
	}
	gotDownload, err := readDownload(dir)
	if err != nil || gotDownload.Target.FileName != edition.FileName || gotDownload.LastURL != "https://b.example/"+stateTestFile {
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
	bad := stateTestEdition()
	bad.FileName = "../../etc/passwd"
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

// A persisted edition must have a plausible size and a hex SHA-256, in both files.
func TestStateFilesValidateEditionSizeAndChecksum(t *testing.T) {
	for name, mutate := range map[string]func(*Edition){
		"zero size":       func(e *Edition) { e.Size = 0 },
		"negative size":   func(e *Edition) { e.Size = -1 },
		"huge size":       func(e *Edition) { e.Size = 1<<63 - 1 },
		"just too large":  func(e *Edition) { e.Size = maxEditionBytes + 1 },
		"no checksum":     func(e *Edition) { e.SHA256 = "" },
		"short checksum":  func(e *Edition) { e.SHA256 = "ab" },
		"upper checksum":  func(e *Edition) { e.SHA256 = strings.Repeat("AB", 32) },
		"non-hex digest":  func(e *Edition) { e.SHA256 = strings.Repeat("zz", 32) },
		"other file name": func(e *Edition) { e.FileName = "notes.txt" },
	} {
		dir := t.TempDir()
		edition := stateTestEdition()
		mutate(&edition)
		if err := writeState(dir, &stateFile{Edition: &edition}); err != nil {
			t.Fatal(err)
		}
		if _, err := readState(dir); err == nil {
			t.Errorf("%s: state.json accepted", name)
		}
		if err := writeDownload(dir, &downloadFile{Target: edition}); err != nil {
			t.Fatal(err)
		}
		if _, err := readDownload(dir); err == nil {
			t.Errorf("%s: download.json accepted", name)
		}
	}
	// The boundary itself is fine.
	dir := t.TempDir()
	edition := stateTestEdition()
	edition.Size = maxEditionBytes
	if err := writeState(dir, &stateFile{Edition: &edition}); err != nil {
		t.Fatal(err)
	}
	if st, err := readState(dir); err != nil || st == nil {
		t.Fatalf("an edition of maxEditionBytes = %+v, %v", st, err)
	}
}

func TestStateFilesIgnoreNewerVersions(t *testing.T) {
	dir := t.TempDir()
	// A future layout may not even decode into today's structs.
	future := `{"version": 2, "edition": "a different shape", "urls": 7}`
	for _, name := range []string{stateFileName, downloadFileName} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(future), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if st, err := readState(dir); st != nil || err != nil {
		t.Fatalf("future state.json = %+v, %v", st, err)
	}
	if d, err := readDownload(dir); d != nil || err != nil {
		t.Fatalf("future download.json = %+v, %v", d, err)
	}
	for _, broken := range []string{`{"version": -1}`, `{"version": "one"}`, `[]`} {
		if err := os.WriteFile(filepath.Join(dir, stateFileName), []byte(broken), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := readState(dir); err == nil {
			t.Errorf("state.json %s was accepted", broken)
		}
	}
	// A file without a version (older or hand-written) still reads.
	edition := stateTestEdition()
	if err := os.WriteFile(filepath.Join(dir, stateFileName), []byte(`{"edition": {"file_name": "`+edition.FileName+`", "size": 42, "sha256": "`+edition.SHA256+`"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if st, err := readState(dir); err != nil || st == nil || st.Edition == nil || st.Edition.Size != 42 {
		t.Fatalf("unversioned state.json = %+v, %v", st, err)
	}
}

func TestStateDropsPendingDeletesThatAreNotEditionFileNames(t *testing.T) {
	dir := t.TempDir()
	edition := stateTestEdition()
	old := "wikipedia_de_all_nopic_2026-09.zim"
	st := &stateFile{Edition: &edition, PendingDelete: []string{
		old, "notes.txt", "../" + old, "sub/" + old, `sub\` + old, "..", "", old, old + ".part", "/etc/passwd",
	}}
	if err := writeState(dir, st); err != nil {
		t.Fatal(err)
	}
	got, err := readState(dir)
	if err != nil || !reflect.DeepEqual(got.PendingDelete, []string{old}) {
		t.Fatalf("pending deletes = %v, %v (want only %s, once)", got, err, old)
	}
	if err := writeState(dir, &stateFile{PendingDelete: []string{"notes.txt"}}); err != nil {
		t.Fatal(err)
	}
	if got, err := readState(dir); err != nil || len(got.PendingDelete) != 0 {
		t.Fatalf("all-invalid pending deletes = %+v, %v", got, err)
	}
}

func TestDownloadFileValidatesMirrorURLs(t *testing.T) {
	good := "https://good.example/zim/" + stateTestFile
	catalog, _ := url.Parse("https://127.0.0.1:8443")
	var many []string
	for i := 0; i <= maxMirrors; i++ {
		many = append(many, good)
	}
	for name, d := range map[string]downloadFile{
		"http":             {URLs: []string{good, "http://bad.example/" + stateTestFile}},
		"userinfo":         {URLs: []string{"https://user:pw@bad.example/" + stateTestFile}},
		"no host":          {URLs: []string{"https:///" + stateTestFile}},
		"other file":       {URLs: []string{"https://bad.example/x.zim"}},
		"loopback":         {URLs: []string{"https://127.0.0.1:9999/m1/" + stateTestFile}},
		"private":          {URLs: []string{"https://192.168.1.2/" + stateTestFile}},
		"bad last url":     {URLs: []string{good}, LastURL: "http://bad.example/" + stateTestFile},
		"too many mirrors": {URLs: many},
	} {
		dir := t.TempDir()
		d.Target = stateTestEdition()
		if err := writeDownload(dir, &d); err != nil {
			t.Fatal(err)
		}
		if _, err := readDownload(dir); err == nil {
			t.Errorf("%s: download.json accepted", name)
		}
	}
	// The trusted catalog host may be local (the fake Kiwix of the tests), other local ports may not.
	dir := t.TempDir()
	d := downloadFile{Target: stateTestEdition(), URLs: []string{"https://127.0.0.1:8443/m1/" + stateTestFile}, LastURL: good}
	if err := writeDownload(dir, &d); err != nil {
		t.Fatal(err)
	}
	if _, err := readDownload(dir); err == nil {
		t.Error("a local mirror was accepted without a trusted host")
	}
	if got, err := readDownload(dir, catalog); err != nil || got == nil || len(got.URLs) != 1 {
		t.Errorf("trusted local mirror = %+v, %v", got, err)
	}
	d.URLs = []string{"https://127.0.0.1:9999/m1/" + stateTestFile}
	if err := writeDownload(dir, &d); err != nil {
		t.Fatal(err)
	}
	if _, err := readDownload(dir, catalog); err == nil {
		t.Error("a local mirror on another port was accepted")
	}
}
