package chibisensei

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"aurago/internal/chibisensei/trainer"
)

// withBaseGGUF returns the outputs of o with a base GGUF of the given architecture and a manifest that matches.
func withBaseGGUF(o fakeRunOutputs, baseArch string) map[string][]byte {
	files := fakeOutputs(o)
	files[OutBaseGGUF] = fakeGGUF(baseArch)
	var m struct {
		Files []struct{ Path, Kind string } `json:"files"`
	}
	_ = json.Unmarshal(files[OutManifest], &m)
	kinds := map[string]string{}
	for _, f := range m.Files {
		kinds[f.Path] = f.Kind
	}
	files[OutManifest] = fakeManifestJSON(o, files, kinds)
	return files
}

// nextRun submits a second run of the fixture's project on the same store and data directory.
func (fx *runFixture) nextRun(files func(o fakeRunOutputs) map[string][]byte) {
	fx.t.Helper()
	o := fx.outOpts
	o.RunID = uuid.NewString()
	fx.f.SetOutputs(files(o))
	in := fx.submit
	in.RunID = o.RunID
	r, err := fx.o.Submit(context.Background(), in)
	if err != nil {
		fx.t.Fatal(err)
	}
	fx.run = r
}

func (fx *runFixture) baseRow() (BaseGGUF, bool) {
	fx.t.Helper()
	lock, err := trainer.LoadLock()
	if err != nil {
		fx.t.Fatal(err)
	}
	b, ok, err := fx.st.GetBaseGGUF(context.Background(), fx.submit.Profile.ID, fx.submit.Profile.Base.Revision,
		BaseGGUFConverter(lock.LlamaCppTag))
	if err != nil {
		fx.t.Fatal(err)
	}
	return b, ok
}

func (fx *runFixture) readChibi(rel string) []byte {
	fx.t.Helper()
	b, err := os.ReadFile(filepath.Join(ChibiDir(fx.dataDir), filepath.FromSlash(rel)))
	if err != nil {
		fx.t.Fatal(err)
	}
	return b
}

func TestOrchestratorSharedBaseGGUFConflict(t *testing.T) {
	fx := newRunFixture(t, fixtureOpts{profile: "qwen35-0p8b", base: true})
	a := fx.drive(nil)
	fx.assertSucceeded(a, restartArtifactKinds)
	row, _ := fx.baseRow()
	before := fx.readChibi(row.RelPath)
	fx.nextRun(func(o fakeRunOutputs) map[string][]byte { return withBaseGGUF(o, "other-arch") })
	b := fx.drive(nil)
	if b.State != RunFailed || b.ErrorCode != "checksum_mismatch" {
		t.Fatalf("run B = %s %s", b.State, b.ErrorCode)
	}
	if got, ok := fx.baseRow(); !ok || got != row {
		t.Errorf("shared row changed: %+v -> %+v", row, got)
	}
	if string(fx.readChibi(row.RelPath)) != string(before) {
		t.Error("shared file changed")
	}
	if got := fx.artifactKinds(a); len(got) != len(restartArtifactKinds) {
		t.Errorf("run A artifacts = %v", got)
	}
}

func TestOrchestratorBaseGGUFArchitectureDrop(t *testing.T) {
	t.Run("own base is removed", func(t *testing.T) {
		fx := newRunFixture(t, fixtureOpts{profile: "qwen35-0p8b", base: true})
		fx.f.SetOutputs(withBaseGGUF(fx.outOpts, "other-arch"))
		r := fx.drive(nil)
		if r.State != RunSucceeded || r.Remote.WarningCode != "base_gguf_invalid" {
			t.Fatalf("run = %s %s warning %q", r.State, r.ErrorCode, r.Remote.WarningCode)
		}
		if _, ok := fx.baseRow(); ok {
			t.Error("base row kept")
		}
		for _, k := range fx.artifactKinds(r) {
			if k == "gguf_base" {
				t.Error("base artifact kept")
			}
		}
		matches, _ := filepath.Glob(filepath.Join(ChibiDir(fx.dataDir), "base", "*.gguf"))
		if len(matches) != 0 {
			t.Errorf("base files left: %v", matches)
		}
	})
	t.Run("reused base stays", func(t *testing.T) {
		fx := newRunFixture(t, fixtureOpts{profile: "qwen35-0p8b", base: true})
		a := fx.drive(nil)
		fx.assertSucceeded(a, restartArtifactKinds)
		// Make the stored base an invalid-architecture file that its row still describes, then let run B reuse it.
		row, _ := fx.baseRow()
		bad := fakeGGUF("other-arch")
		if err := os.WriteFile(filepath.Join(ChibiDir(fx.dataDir), filepath.FromSlash(row.RelPath)), bad, 0o600); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(bad)
		row.SHA256 = hex.EncodeToString(sum[:])
		if err := fx.st.PutBaseGGUF(context.Background(), row); err != nil {
			t.Fatal(err)
		}
		fx.nextRun(func(o fakeRunOutputs) map[string][]byte { return withBaseGGUF(o, "other-arch") })
		r := fx.drive(nil)
		if r.State != RunSucceeded || r.Remote.WarningCode != "base_gguf_invalid" {
			t.Fatalf("run = %s %s warning %q", r.State, r.ErrorCode, r.Remote.WarningCode)
		}
		if got, ok := fx.baseRow(); !ok || got != row {
			t.Errorf("shared row = %+v %v", got, ok)
		}
		if string(fx.readChibi(row.RelPath)) != string(bad) {
			t.Error("shared file changed")
		}
		for _, k := range fx.artifactKinds(r) {
			if k == "gguf_base" {
				t.Error("this run's base artifact kept")
			}
		}
	})
}

// staleStagedDriver keeps an existing staged file of the status output in place, as a crash between the copy commit
// and the removal of the staged file would.
type simulatedCrash struct{}

type staleStagedDriver struct {
	*fakeKaggle
	armed bool
}

func (d *staleStagedDriver) RemoveFile(project, name string) error {
	if d.armed && strings.HasSuffix(name, "-"+OutStatus) {
		if f, err := d.fakeKaggle.OpenFile(project, name); err == nil {
			f.Close()
			panic(simulatedCrash{})
		}
	}
	return d.fakeKaggle.RemoveFile(project, name)
}

func TestOrchestratorRestartAfterCopyBeforeRemoval(t *testing.T) {
	fx := newRunFixture(t, fixtureOpts{profile: "qwen35-0p8b", base: true})
	d := &staleStagedDriver{fakeKaggle: fx.f, armed: true}
	fx.o = NewOrchestrator(fx.st, fx.hub, fx.cat, d, fx.dataDir, func() OrchestratorConfig { return fx.cfg })
	fx.o.now = fx.clk.Now
	crashed := false
	func() {
		defer func() {
			if v := recover(); v != nil {
				if _, ok := v.(simulatedCrash); !ok {
					panic(v)
				}
				crashed = true
			}
		}()
		fx.drive(nil)
	}()
	if !crashed {
		t.Fatal("the removal of the staged status file was never reached")
	}
	r := fx.get()
	if o := findOutput(r.Remote.Outputs, OutStatus); o == nil || !o.Copied {
		t.Fatalf("status not copied in %s", r.State)
	}
	if !slices.ContainsFunc(fx.f.ProjectFiles(), func(n string) bool { return strings.HasSuffix(n, "-"+OutStatus) }) {
		t.Fatal("the staged status file is gone, the crash was not simulated")
	}
	fx.restart()
	r = fx.drive(nil)
	fx.assertSucceeded(r, restartArtifactKinds)
	assertMutations(t, fx.f, fx.happyMutations(r))
	fx.assertNoReplays(r, true)
}
