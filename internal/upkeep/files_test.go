package upkeep

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/webassets"
)

func TestPinFromRealBuildMetadataWithoutExecution(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip(err)
	}
	d := t.TempDir()
	files := map[string]string{
		"go.mod":                    "module aurago\n\ngo 1.26\n",
		"internal/webassets/pin.go": "package webassets\nvar SetID string\n",
		"cmd/aurago/main.go":        "package main\nimport (\"aurago/internal/webassets\";\"fmt\")\nfunc main(){panic(fmt.Sprint(webassets.SetID))}\n",
	}
	for p, v := range files {
		p = filepath.Join(d, p)
		os.MkdirAll(filepath.Dir(p), 0700)
		os.WriteFile(p, []byte(v), 0600)
	}
	id := strings.Repeat("a", 64)
	target := filepath.Join(d, "fixture.exe")
	c := exec.Command("go", "build", "-ldflags=-X aurago/internal/webassets.SetID="+id, "-o", target, "./cmd/aurago")
	c.Dir = d
	c.Env = append(os.Environ(), "GOWORK=off")
	if b, e := c.CombinedOutput(); e != nil {
		t.Fatalf("fixture build: %v %s", e, b)
	}
	got, err := PinFromBinary(target)
	if err != nil || got != id {
		t.Fatalf("pin=%q error=%v", got, err)
	}
}

func TestReleaseMarkerAndRecentBuildProtection(t *testing.T) {
	f := newFixture(t)
	old := f.asset(9)
	f.archive(old)
	if e := writeJSON(filepath.Join(f.root, "deploy", "aurago-web-assets-"+old+".release.json"), webassets.Pin{ID: old, URL: "https://example.invalid/releases/v1/assets"}); e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(f.root, "assets", "web", old)
	past := time.Now().Add(-48 * time.Hour)
	os.Chtimes(p, past, past)
	recent := f.asset(10)
	if _, e := f.clean(); e != nil {
		t.Fatal(e)
	}
	for _, id := range []string{old, recent} {
		if e := verifyAssets(filepath.Join(f.root, "assets", "web"), id); e != nil {
			t.Fatal("protected resource deleted", e)
		}
	}
	archive := filepath.Join(f.root, "deploy", "aurago-web-assets-"+old+".tar.gz")
	if _, e := os.Stat(archive); e != nil {
		t.Fatal("release deleted")
	}
	recentPath := filepath.Join(f.root, "assets", "web", recent)
	os.Chtimes(recentPath, past, past)
	if _, e := f.clean(); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(recentPath); !os.IsNotExist(e) {
		t.Fatal("old unreferenced build not adopted")
	}
}

func TestDeletionRejectsChangedIdentityAndTraversal(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "candidate")
	os.WriteFile(p, []byte("old"), 0600)
	old, _ := snapshotInfo(p)
	os.Rename(p, p+".old")
	os.WriteFile(p, []byte("new"), 0600)
	r, e := os.OpenRoot(d)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	if e := removeTree(r, "candidate", old); e == nil {
		t.Fatal("changed candidate accepted")
	}
	if e := removeTree(r, "../candidate", old); e == nil {
		t.Fatal("traversal accepted")
	}
	if b, e := os.ReadFile(p); e != nil || string(b) != "new" {
		t.Fatal("replacement was deleted")
	}
}

func TestMaintenanceCLIFlagsAndPendingCheck(t *testing.T) {
	if code := RunCLI([]string{"--help"}, io.Discard, io.Discard); code != 0 {
		t.Fatal(code)
	}
	if code := RunCLI(nil, io.Discard, io.Discard); code != 2 {
		t.Fatal("missing root accepted")
	}
	f := newFixture(t)
	if code := RunCLI([]string{"--root", f.root, "--check-pending"}, io.Discard, io.Discard); code != 0 {
		t.Fatal(code)
	}
	id := f.update(1, false)
	p := filepath.Join(f.root, StateDir, "transactions", id, "manifest.json")
	var tx Transaction
	readJSON(p, &tx)
	tx.Status = "uncertain"
	writeJSON(p, tx)
	if code := RunCLI([]string{"--root", f.root, "--check-pending"}, io.Discard, io.Discard); code != 1 {
		t.Fatal("uncertain transaction allowed another update")
	}
	if code := RunCLI([]string{"--root", f.root, "--resolve", id, "--outcome", "confirmed"}, io.Discard, io.Discard); code != 2 {
		t.Fatal("resolution without --apply accepted")
	}
}
