package chibisensei

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"

	"aurago/internal/chibisensei/trainer"
)

const trainerPlaceholderMarker = "CHIBI_TRAINER_PLACEHOLDER = True"

type proofBundleFile struct {
	Name   string `json:"name"`
	Role   string `json:"role"`   // train | val | eval | marker | job | notebook
	Upload bool   `json:"upload"` // part of the Kaggle dataset
	Rows   int    `json:"rows"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type proofBundle struct {
	V                     int               `json:"v"`
	RunID                 string            `json:"run_id"`
	Run12                 string            `json:"run12"`
	ProfileID             string            `json:"profile_id"`
	ProfileSHA            string            `json:"profile_sha"`
	Recipe                string            `json:"recipe"`
	Preset                string            `json:"preset"`
	Method                string            `json:"method"`
	Owner                 string            `json:"owner"` // CHIBI_PROOF_OWNER, empty when unset
	PlaceholderTrainer    bool              `json:"placeholder_trainer"`
	DatasetSlug           string            `json:"dataset_slug"`
	NotebookSlug          string            `json:"notebook_slug"`
	MachineShape          string            `json:"machine_shape"`
	SessionTimeoutSeconds int               `json:"session_timeout_seconds"`
	EstimateMinutes       int               `json:"estimate_minutes"`
	MaxTrainSeconds       int               `json:"max_train_seconds"`
	ScriptSHA256          string            `json:"script_sha256"`
	DataSHA               string            `json:"data_sha"`
	Files                 []proofBundleFile `json:"files"`
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func writeBundleFile(t *testing.T, dir, name, role string, data []byte) proofBundleFile {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return proofBundleFile{Name: name, Role: role, Bytes: int64(len(data)), SHA256: hex.EncodeToString(sum[:])}
}

// TestProofBundle writes the files of one manual proof run (contract, "Proof runs" step 2). It is skipped
// unless CHIBI_PROOF_OUT is set.
func TestProofBundle(t *testing.T) {
	out := os.Getenv("CHIBI_PROOF_OUT")
	if out == "" {
		t.Skip("set CHIBI_PROOF_OUT=<dir> and CHIBI_PROOF_PROFILE=<id> to build a proof bundle")
	}
	profileID := os.Getenv("CHIBI_PROOF_PROFILE")
	c := mustCatalog(t)
	if _, ok := c.Get(profileID); !ok {
		t.Fatalf("CHIBI_PROOF_PROFILE %q is not a profile", profileID)
	}
	recipe := envOr("CHIBI_PROOF_RECIPE", proofDefaultRecipe(profileID))
	preset := envOr("CHIBI_PROOF_PRESET", "quick")
	timeout, err := strconv.Atoi(envOr("CHIBI_PROOF_TIMEOUT_MINUTES", "60"))
	if err != nil || timeout < 30 || timeout > 720 {
		t.Fatalf("CHIBI_PROOF_TIMEOUT_MINUTES must be 30 to 720")
	}
	script := trainer.Script()
	placeholder := bytes.Contains(script, []byte(trainerPlaceholderMarker))
	if placeholder && os.Getenv("CHIBI_PROOF_ALLOW_PLACEHOLDER") != "1" {
		t.Fatal("the embedded trainer is still the placeholder (CHIBI_PROOF_ALLOW_PLACEHOLDER=1 only checks the bundle path)")
	}
	lock, err := trainer.LoadLock()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	st, err := OpenStore(filepath.Join(t.TempDir(), "chibi_sensei.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	run := proofPrepare(t, st, c, profileID, recipe, preset, timeout)
	proj, plan, checks, flags := run.proj, run.plan, run.checks, run.flags
	if !checks.Ready {
		t.Fatalf("dataset not ready: %v", run.blocks())
	}
	if err := st.SaveChecks(ctx, checks, flags); err != nil {
		t.Fatal(err)
	}
	runID := uuid.NewString()
	snap, err := st.FreezeSnapshot(ctx, proj.ID, runID, out)
	if err != nil {
		t.Fatal(err)
	}
	if snap.DataRev != plan.DataRev {
		t.Fatalf("snapshot data_rev %d, plan %d", snap.DataRev, plan.DataRev)
	}
	spec, err := BuildJobSpec(JobInput{RunID: runID, Created: time.Now().UTC().Truncate(time.Second),
		AuraGoVersion: "proof-bundle", Project: proj, Profile: run.prof, ProfileSHA: c.SHA(profileID), Plan: plan,
		Marker: snap.Marker.Name, Format: snap.Format, Lock: lock})
	if err != nil {
		t.Fatal(err)
	}
	job, err := spec.JSON()
	if err != nil {
		t.Fatal(err)
	}
	nb, err := BuildNotebook(runID, job, script)
	if err != nil {
		t.Fatal(err)
	}
	if f := append(jobFindings(job), notebookFindings(nb, job, script)...); len(f) > 0 {
		t.Fatalf("payload check: %v", f)
	}
	run12 := Run12(runID)
	b := proofBundle{V: 1, RunID: runID, Run12: run12, ProfileID: profileID, ProfileSHA: c.SHA(profileID),
		Recipe: recipe, Preset: plan.Preset, Method: plan.Method, DatasetSlug: DatasetSlug(runID),
		NotebookSlug: NotebookSlug(proj.ID, runID), MachineShape: envOr("CHIBI_PROOF_MACHINE_SHAPE", "NvidiaTeslaT4"), Owner: os.Getenv("CHIBI_PROOF_OWNER"), PlaceholderTrainer: placeholder,
		SessionTimeoutSeconds: plan.SessionTimeoutMinutes * 60, EstimateMinutes: plan.EstimateMinutes,
		MaxTrainSeconds: plan.MaxTrainSeconds, ScriptSHA256: trainer.ScriptSHA256(), DataSHA: snap.DataSHA}
	for _, split := range SnapshotSplits {
		if f, ok := snap.Files[split]; ok {
			b.Files = append(b.Files, proofBundleFile{Name: f.Name, Role: split, Upload: true, Rows: f.Rows, Bytes: f.Bytes, SHA256: f.SHA256})
		}
	}
	b.Files = append(b.Files, proofBundleFile{Name: snap.Marker.Name, Role: "marker", Upload: true,
		Bytes: snap.Marker.Bytes, SHA256: snap.Marker.SHA256})
	b.Files = append(b.Files, writeBundleFile(t, out, "r"+run12+"-job.json", "job", job))
	b.Files = append(b.Files, writeBundleFile(t, out, "r"+run12+"-notebook.ipynb", "notebook", nb))
	summary, err := marshalStable(b)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "bundle.json"), summary, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Logf("proof bundle %s for %s (%s, %s): %d files in %s", runID, profileID, recipe, plan.Preset, len(b.Files), out)
}
