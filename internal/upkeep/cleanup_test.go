package upkeep

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aurago/internal/webassets"
	"github.com/gofrs/flock"
)

type fixture struct {
	t       *testing.T
	root    string
	options Options
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{t: t, root: t.TempDir()}
	f.write("config.yaml", []byte("fixture: true"))
	for _, d := range []string{"assets/web", "deploy", StateDir + "/transactions"} {
		if err := os.MkdirAll(filepath.Join(f.root, d), 0700); err != nil {
			t.Fatal(err)
		}
	}
	f.options = Options{Root: f.root, Apply: true, AdoptLegacy: true, LegacyDir: t.TempDir(), ReadPin: func(p string) (string, error) {
		b, e := os.ReadFile(p)
		if e != nil {
			return "", e
		}
		parts := strings.Split(string(b), "|")
		if len(parts) != 2 || !hashPattern.MatchString(parts[0]) {
			return "", fmt.Errorf("bad fixture binary")
		}
		return parts[0], nil
	}, Health: func(context.Context, string) error { return nil }}
	f.install(0, false)
	return f
}

func (f *fixture) write(rel string, b []byte) {
	f.t.Helper()
	p := filepath.Join(f.root, filepath.FromSlash(rel))
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		f.t.Fatal(e)
	}
	if e := os.WriteFile(p, b, 0600); e != nil {
		f.t.Fatal(e)
	}
}
func (f *fixture) asset(n int) string {
	f.t.Helper()
	data := []byte(fmt.Sprintf("resource %d", n))
	m := webassets.Manifest{Version: 1, Files: []webassets.Entry{{Path: "index.html", Size: int64(len(data)), SHA256: webassets.Digest(data)}}}
	raw, _ := json.Marshal(m)
	id := webassets.Digest(raw)
	f.write("assets/web/"+id+"/"+webassets.ManifestName, raw)
	f.write("assets/web/"+id+"/index.html", data)
	return id
}
func (f *fixture) install(n int, shared bool) string {
	f.t.Helper()
	assetN := n
	if shared {
		assetN = 0
	}
	id := f.asset(assetN)
	f.write("bin/aurago_linux", []byte(fmt.Sprintf("%s|binary-%d", id, n)))
	return id
}
func (f *fixture) update(n int, shared bool) string {
	f.t.Helper()
	old, err := os.ReadFile(filepath.Join(f.root, "bin/aurago_linux"))
	if err != nil {
		f.t.Fatal(err)
	}
	id := fmt.Sprintf("txn-%06d", n)
	p := filepath.Join(f.root, StateDir, "transactions", id)
	f.write(StateDir+"/transactions/"+id+"/bin/aurago_linux", old)
	f.write(StateDir+"/transactions/"+id+"/data/fixture.db", []byte("backup data"))
	h, _ := digestFile(filepath.Join(p, "bin", "aurago_linux"))
	oldID, _ := f.options.ReadPin(filepath.Join(p, "bin", "aurago_linux"))
	newID := f.install(n, shared)
	t := Transaction{Version: 1, Root: f.root, ID: id, Created: int64(n), Status: "confirmed", PreviousVersion: h, PreviousAsset: oldID, NewAsset: newID, BackupComplete: true}
	if e := writeJSON(filepath.Join(p, "manifest.json"), t); e != nil {
		f.t.Fatal(e)
	}
	return id
}
func (f *fixture) clean() (Report, error) {
	f.t.Helper()
	return Cleanup(context.Background(), f.options)
}
func (f *fixture) archive(id string) {
	f.t.Helper()
	raw, e := os.ReadFile(filepath.Join(f.root, "assets", "web", id, webassets.ManifestName))
	if e != nil {
		f.t.Fatal(e)
	}
	var b bytes.Buffer
	g := gzip.NewWriter(&b)
	a := tar.NewWriter(g)
	a.WriteHeader(&tar.Header{Name: webassets.ManifestName, Mode: 0644, Size: int64(len(raw)), Typeflag: tar.TypeReg})
	a.Write(raw)
	var manifest webassets.Manifest
	if e := json.Unmarshal(raw, &manifest); e != nil {
		f.t.Fatal(e)
	}
	for _, entry := range manifest.Files {
		payload, e := os.ReadFile(filepath.Join(f.root, "assets", "web", id, filepath.FromSlash(entry.Path)))
		if e != nil {
			f.t.Fatal(e)
		}
		a.WriteHeader(&tar.Header{Name: entry.Path, Mode: 0644, Size: int64(len(payload)), Typeflag: tar.TypeReg})
		a.Write(payload)
	}
	a.Close()
	g.Close()
	f.write("deploy/aurago-web-assets-"+id+".tar.gz", b.Bytes())
}

func TestTwelveUpdatesRemainBounded(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprint(shared), func(t *testing.T) {
			f := newFixture(t)
			f.write("data/models/model.bin", []byte("model"))
			f.write("backups/manual/notes", []byte("manual"))
			f.write("data/homepage/project", []byte("project"))
			for n := 1; n <= 12; n++ {
				f.update(n, shared)
				pin, _ := f.options.ReadPin(filepath.Join(f.root, "bin", "aurago_linux"))
				f.archive(pin)
				if _, e := f.clean(); e != nil {
					t.Fatal(e)
				}
				transactions, _ := os.ReadDir(filepath.Join(f.root, StateDir, "transactions"))
				if len(transactions) > 2 {
					t.Fatalf("%d backups after update %d", len(transactions), n)
				}
				sets, _ := os.ReadDir(filepath.Join(f.root, "assets", "web"))
				count := 0
				for _, s := range sets {
					if hashPattern.MatchString(s.Name()) {
						count++
					}
				}
				if count > 3 || (shared && count != 1) {
					t.Fatalf("unbounded resource sets: %d", count)
				}
				archives, _ := filepath.Glob(filepath.Join(f.root, "deploy", "*.tar.gz"))
				if len(archives) > 3 {
					t.Fatalf("unbounded archives: %d", len(archives))
				}
			}
			for _, p := range []string{"data/models/model.bin", "backups/manual/notes", "data/homepage/project"} {
				if _, e := os.Stat(filepath.Join(f.root, p)); e != nil {
					t.Fatal(e)
				}
			}
			r, e := f.clean()
			if e != nil || r.Freed != 0 {
				t.Fatalf("cleanup not idempotent: %+v %v", r, e)
			}
		})
	}
}

func TestDryRunAndReadinessNeverDelete(t *testing.T) {
	f := newFixture(t)
	for i := 1; i <= 4; i++ {
		f.update(i, false)
	}
	f.options.Apply = false
	r, e := f.clean()
	if e != nil {
		t.Fatal(e)
	}
	candidates := 0
	for _, v := range r.Entries {
		if v.Action == "delete" {
			candidates++
		}
	}
	if candidates == 0 || r.Freed != 0 {
		t.Fatal("invalid preview")
	}
	if _, e := ReadResult(f.root); !os.IsNotExist(e) {
		t.Fatal("preview wrote an outcome")
	}
	f.options.Apply = true
	f.options.Health = func(context.Context, string) error { return errors.New("not ready") }
	if _, e := f.clean(); e == nil {
		t.Fatal("accepted failed readiness")
	}
	for _, v := range r.Entries {
		if v.Action == "delete" {
			if _, e := os.Stat(v.Path); e != nil {
				t.Fatal("deleted before readiness")
			}
		}
	}
}

func TestUnsafeStateBlocksCollection(t *testing.T) {
	for _, kind := range []string{"pending", "uncertain", "incomplete", "foreign", "corrupt", "bad_backup", "bad_current"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture(t)
			for i := 1; i <= 4; i++ {
				f.update(i, false)
			}
			p := filepath.Join(f.root, StateDir, "transactions", "txn-000004", "manifest.json")
			var tx Transaction
			if e := readJSON(p, &tx); e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "pending", "uncertain":
				tx.Status = kind
			case "incomplete":
				tx.BackupComplete = false
			case "foreign":
				tx.Root = t.TempDir()
			case "bad_backup":
				f.write(StateDir+"/transactions/txn-000004/bin/aurago_linux", []byte("corrupt"))
			case "bad_current":
				id, _ := f.options.ReadPin(filepath.Join(f.root, "bin", "aurago_linux"))
				f.write("assets/web/"+id+"/index.html", []byte("bad"))
			}
			if e := writeJSON(p, tx); e != nil {
				t.Fatal(e)
			}
			if kind == "corrupt" {
				os.WriteFile(p, []byte("{"), 0600)
			}
			if _, e := f.clean(); e == nil {
				t.Fatal("unsafe state accepted")
			}
			if _, e := os.Stat(filepath.Join(f.root, StateDir, "transactions", "txn-000001")); e != nil {
				t.Fatal("deleted during unsafe state")
			}
		})
	}
}

func TestUpdateAndAssetLocks(t *testing.T) {
	f := newFixture(t)
	unlock, e := lock(f.root)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := f.clean(); e == nil {
		t.Fatal("cleanup raced update lock")
	}
	unlock()
	al := flock.New(filepath.Join(f.root, "assets", "web", ".install.lock"))
	if e := al.Lock(); e != nil {
		t.Fatal(e)
	}
	defer al.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := Cleanup(ctx, f.options); e == nil {
		t.Fatal("cleanup ignored asset lock/cancellation")
	}
}

func TestExplicitBinaryProtectsAssets(t *testing.T) {
	f := newFixture(t)
	old, _ := os.ReadFile(filepath.Join(f.root, "bin", "aurago_linux"))
	oldID, _ := f.options.ReadPin(filepath.Join(f.root, "bin", "aurago_linux"))
	f.write("bin/aurago_linux.prev", old)
	for i := 1; i <= 5; i++ {
		f.update(i, false)
	}
	if _, e := f.clean(); e != nil {
		t.Fatal(e)
	}
	if e := verifyAssets(filepath.Join(f.root, "assets", "web"), oldID); e != nil {
		t.Fatal("explicit rollback lost", e)
	}
}

func TestLegacyRequiresInstallationBinding(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux legacy backups")
	}
	f := newFixture(t)
	for i := 1; i <= 5; i++ {
		id := f.update(i, false)
		source := filepath.Join(f.root, StateDir, "transactions", id)
		dest := filepath.Join(f.options.LegacyDir, fmt.Sprintf("aurago-backup-%d", i))
		if e := os.Rename(source, dest); e != nil {
			t.Fatal(e)
		}
		os.Remove(filepath.Join(dest, "manifest.json"))
		marker := filepath.Join(f.root, "data", "tsnet")
		if i == 1 {
			marker = filepath.Join(t.TempDir(), "data", "tsnet")
		}
		os.WriteFile(filepath.Join(dest, "tsnet-state.path"), []byte(marker), 0600)
	}
	if _, e := f.clean(); e != nil {
		t.Fatal(e)
	}
	entries, _ := os.ReadDir(f.options.LegacyDir)
	if len(entries) != 1 {
		t.Fatalf("wanted only foreign backup after adopting two rollbacks: %d", len(entries))
	}
	adopted, _ := os.ReadDir(filepath.Join(f.root, StateDir, "transactions"))
	if len(adopted) != 2 {
		t.Fatal("legacy rollbacks were not durably adopted")
	}
	if _, e := os.Stat(filepath.Join(f.options.LegacyDir, "aurago-backup-1")); e != nil {
		t.Fatal("foreign backup deleted")
	}
}

func TestSymlinksCannotEscape(t *testing.T) {
	f := newFixture(t)
	outside := t.TempDir()
	target := filepath.Join(outside, "precious")
	os.WriteFile(target, []byte("keep"), 0600)
	id := f.asset(9)
	link := filepath.Join(f.root, "assets", "web", id, "external")
	if e := os.Symlink(outside, link); e != nil {
		t.Skip(e)
	}
	r, e := f.clean()
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range r.Entries {
		if entry.Path == filepath.Dir(link) && entry.Action != "keep" {
			t.Fatal("linked tree deleted")
		}
	}
	if _, e := os.Stat(target); e != nil {
		t.Fatal("escaped resource root")
	}
	cache := filepath.Join(f.root, StateDir, "go-cache")
	if e := os.Symlink(outside, cache); e != nil {
		t.Fatal(e)
	}
	if _, e := f.clean(); e == nil {
		t.Fatal("linked cache accepted")
	}
}

func TestCacheThresholdAndResults(t *testing.T) {
	f := newFixture(t)
	f.write(StateDir+"/go-cache/sparse", nil)
	file, e := os.OpenFile(filepath.Join(f.root, StateDir, "go-cache", "sparse"), os.O_WRONLY, 0600)
	if e != nil {
		t.Fatal(e)
	}
	if e = file.Truncate(CacheLimit + 1); e != nil {
		file.Close()
		t.Skip(e)
	}
	file.Close()
	if _, e := f.clean(); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Join(f.root, StateDir, "go-cache")); !os.IsNotExist(e) {
		t.Fatal("oversized cache retained")
	}
	state, e := ReadResult(f.root)
	if e != nil || !state.Success {
		t.Fatal(state, e)
	}
	f.options.Health = func(context.Context, string) error { return errors.New("down") }
	f.clean()
	f.clean()
	state, e = ReadResult(f.root)
	if e != nil || state.Failures != 2 || state.Success {
		t.Fatal(state, e)
	}
	f.options.Health = func(context.Context, string) error { return nil }
	f.clean()
	state, _ = ReadResult(f.root)
	if state.Failures != 0 || !state.Success {
		t.Fatal("recovery did not reset failures")
	}
}

func TestRolledBackFailureDoesNotDisplaceGoodVersions(t *testing.T) {
	f := newFixture(t)
	f.update(1, false)
	f.update(2, false)
	f.clean()
	old, _ := os.ReadFile(filepath.Join(f.root, "bin", "aurago_linux"))
	id := f.update(3, false)
	p := filepath.Join(f.root, StateDir, "transactions", id, "manifest.json")
	var tx Transaction
	readJSON(p, &tx)
	tx.Status = "rolled_back"
	writeJSON(p, tx)
	f.write("bin/aurago_linux", old)
	if _, e := f.clean(); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Dir(p)); !os.IsNotExist(e) {
		t.Fatal("failed transaction retained after verified rollback")
	}
	for _, n := range []int{1, 2} {
		if _, e := os.Stat(filepath.Join(f.root, StateDir, "transactions", fmt.Sprintf("txn-%06d", n))); e != nil {
			t.Fatal("successful rollback displaced", e)
		}
	}
}
