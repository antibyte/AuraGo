package trainer_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/chibisensei"
	"aurago/internal/chibisensei/trainer"

	"github.com/google/uuid"
)

// findPython returns python3 or python (3.10+), or skips the test (D19).
func findPython(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"python3", "python"} {
		p, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		out, err := exec.Command(p, "-I", "-c", "import sys; print(sys.version_info[0]*100+sys.version_info[1])").Output()
		if err != nil {
			continue
		}
		if v, err := strconv.Atoi(strings.TrimSpace(string(out))); err == nil && v >= 310 {
			return p
		}
	}
	t.Skip("no Python 3.10+ (python3 or python) on PATH")
	return ""
}

func writeScript(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "chibi_train.py")
	if err := os.WriteFile(path, trainer.Script(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func runPython(t *testing.T, py string, env []string, args ...string) ([]byte, []byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, py, append([]string{"-I"}, args...)...)
	cmd.Env = append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func tail(b []byte) string {
	if len(b) > 4000 {
		b = b[len(b)-4000:]
	}
	return string(b)
}

func TestTrainerSelftest(t *testing.T) {
	py := findPython(t)
	stdout, stderr, err := runPython(t, py, nil, writeScript(t), "--selftest")
	if err != nil || !strings.Contains(string(stdout), "selftest ok") {
		t.Fatalf("selftest: %v\nstdout:\n%s\nstderr:\n%s", err, tail(stdout), tail(stderr))
	}
}

func TestTrainerDryRunNeedsFakeEnv(t *testing.T) {
	py := findPython(t)
	dir := t.TempDir()
	_, _, err := runPython(t, py, []string{"CHIBI_FAKE_ENV=0"}, writeScript(t), "--dry-run",
		"--job", filepath.Join(dir, "job.json"), "--out", filepath.Join(dir, "out"))
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 2 {
		t.Fatalf("dry run without CHIBI_FAKE_ENV=1 must exit 2, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "out")); !os.IsNotExist(err) {
		t.Fatalf("a refused dry run must not create the output directory")
	}
}

type dryCase struct {
	name, profile, recipe, taskKind string
	labels                          []string
	rows                            int
	method, format                  string
}

var dryCases = []dryCase{
	{name: "qwen-lora-assistant", profile: "qwen35-0p8b", recipe: chibisensei.RecipeAssistant, rows: 60, method: "lora", format: "messages"},
	{name: "lfm-lora-assistant", profile: "lfm25-1p2b", recipe: chibisensei.RecipeAssistant, rows: 60, method: "lora", format: "messages"},
	{name: "gemma-full-classify", profile: "gemma3-270m", recipe: chibisensei.RecipeSpecialist, taskKind: "classify",
		labels: []string{"pos", "neg"}, rows: 240, method: "full", format: "messages"},
	{name: "qwen-lora-free", profile: "qwen35-0p8b", recipe: chibisensei.RecipeFree, rows: 60, method: "lora", format: "text"},
}

func caseRows(c dryCase) []chibisensei.RowInput {
	out := make([]chibisensei.RowInput, 0, c.rows)
	for i := 0; i < c.rows; i++ {
		switch {
		case c.recipe == chibisensei.RecipeFree:
			s := fmt.Sprintf("Note %d: the quick brown fox number %d jumps over the lazy dog and keeps running.", i, i*7)
			out = append(out, chibisensei.RowInput{Text: &s})
		case c.taskKind == "classify":
			label := c.labels[i%len(c.labels)]
			out = append(out, chibisensei.RowInput{Messages: []chibisensei.Message{
				{Role: "user", Content: fmt.Sprintf("Review %d: this product is %s.", i, label)},
				{Role: "assistant", Content: label}}})
		default:
			var msgs []chibisensei.Message
			if i%3 == 0 {
				msgs = append(msgs, chibisensei.Message{Role: "system", Content: "Antworte kurz und freundlich."})
			}
			msgs = append(msgs, chibisensei.Message{Role: "user", Content: fmt.Sprintf("Frage %d: Wie spät ist es?", i)},
				chibisensei.Message{Role: "assistant", Content: fmt.Sprintf("Antwort %d: Zeit für eine Pause. ☕", i)})
			out = append(out, chibisensei.RowInput{Messages: msgs})
		}
	}
	return out
}

type dryJob struct {
	runID string
	snap  chibisensei.Snapshot
	plan  chibisensei.Plan
	job   chibisensei.JobSpec
}

func buildDryJob(t *testing.T, cat *chibisensei.Catalog, lock trainer.Lock, c dryCase, inputRoot string) dryJob {
	t.Helper()
	ctx := context.Background()
	st, err := chibisensei.OpenStore(filepath.Join(t.TempDir(), "chibi_sensei.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	proj, err := st.CreateProject(ctx, chibisensei.ProjectInput{Name: c.name, Recipe: c.recipe, TaskKind: c.taskKind,
		Labels: c.labels, GoalText: "dry run", ProfileID: c.profile})
	if err != nil {
		t.Fatal(err)
	}
	rows := caseRows(c)
	for start := 0; start < len(rows); start += 100 {
		end := min(start+100, len(rows))
		if _, err := st.AddRows(ctx, proj.ID, rows[start:end], chibisensei.AddOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	srows, proj, err := st.LoadSplitRows(ctx, proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	prof, ok := cat.Get(c.profile)
	if !ok {
		t.Fatalf("profile %s missing", c.profile)
	}
	split := chibisensei.ComputeSplit(srows, proj.SplitSeed, proj.TaskKind == "classify")
	plan, err := chibisensei.ComputePlan(chibisensei.PlanInput{Project: proj, Profile: prof, Rows: srows, Split: split,
		RemoteTimeoutMinutes: 120, NeedBaseGGUF: true})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Method != c.method {
		t.Fatalf("plan method %s, want %s", plan.Method, c.method)
	}
	runID := uuid.NewString()
	snapDir := filepath.Join(inputRoot, "chibi-data-"+chibisensei.Run12(runID))
	if err := os.MkdirAll(snapDir, 0o700); err != nil {
		t.Fatal(err)
	}
	snap, err := st.FreezeSnapshot(ctx, proj.ID, runID, snapDir)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Format != c.format {
		t.Fatalf("snapshot format %s, want %s", snap.Format, c.format)
	}
	job, err := chibisensei.BuildJobSpec(chibisensei.JobInput{RunID: runID, Created: time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC),
		AuraGoVersion: "test", Project: proj, Profile: prof, ProfileSHA: cat.SHA(c.profile), Plan: plan,
		Marker: snap.Marker.Name, Format: snap.Format, Lock: lock})
	if err != nil {
		t.Fatal(err)
	}
	return dryJob{runID: runID, snap: snap, plan: plan, job: job}
}

func TestTrainerDryRun(t *testing.T) {
	py := findPython(t)
	script := writeScript(t)
	cat, err := chibisensei.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	lock, err := trainer.LoadLock()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range dryCases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "input")
			dj := buildDryJob(t, cat, lock, c, input)
			body, err := dj.job.JSON()
			if err != nil {
				t.Fatal(err)
			}
			jobPath, out := filepath.Join(dir, "job.json"), filepath.Join(dir, "out")
			if err := os.WriteFile(jobPath, body, 0o600); err != nil {
				t.Fatal(err)
			}
			stdout, stderr, err := runPython(t, py, []string{"CHIBI_FAKE_ENV=1"}, script, "--dry-run",
				"--job", jobPath, "--out", out, "--input", input)
			if err != nil {
				t.Fatalf("dry run: %v\nstdout:\n%s\nstderr:\n%s", err, tail(stdout), tail(stderr))
			}
			checkDryRunOutputs(t, c, dj, out, stdout)
		})
	}
}

var trainerStages = []string{"boot", "env_check", "data", "install", "load_base", "converter",
	"base_eval", "train", "tuned_eval", "save", "gguf", "manifest"}

// checkDryRunOutputs parses every output of one dry run with the production parsers.
func checkDryRunOutputs(t *testing.T, c dryCase, dj dryJob, out string, stdout []byte) {
	t.Helper()
	read := func(name string) []byte {
		b, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return b
	}
	st, err := chibisensei.ParseStatus(read("chibi-status.json"))
	if err != nil {
		t.Fatal(err)
	}
	if st.State != "succeeded" || st.RunID != dj.runID {
		t.Fatalf("status: %+v", st)
	}
	if c.method == "lora" && st.Discovery["lora_targets"] == nil {
		t.Fatalf("status.discovery.lora_targets missing")
	}
	man, err := chibisensei.ParseManifest(read("chibi-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if man.RunID != dj.runID || man.ProfileID != c.profile {
		t.Fatalf("manifest header: %+v", man)
	}
	kinds := map[string]bool{}
	for _, f := range man.Files {
		b := read(f.Path)
		sum := sha256.Sum256(b)
		if hex.EncodeToString(sum[:]) != f.SHA256 || int64(len(b)) != f.Bytes {
			t.Fatalf("manifest entry %s does not match the file", f.Path)
		}
		kinds[f.Kind] = true
	}
	want := []string{"gguf_tuned", "metrics", "eval_samples", "status"}
	if c.method == "lora" {
		want = append(want, "adapter", "adapter_config")
	}
	for _, k := range want {
		if !kinds[k] {
			t.Fatalf("manifest lacks kind %s: %v", k, kinds)
		}
	}
	mf, err := os.Open(filepath.Join(out, "chibi-metrics.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer mf.Close()
	metrics, err := chibisensei.ParseMetricsFile(mf)
	if err != nil || len(metrics) == 0 {
		t.Fatalf("metrics: %d, %v", len(metrics), err)
	}
	last := metrics[len(metrics)-1]
	for i := len(metrics) - 1; i >= 0 && last.Kind != "train"; i-- {
		last = metrics[i]
	}
	if last.Kind != "train" || last.Step != last.MaxSteps || last.MaxSteps < 1 {
		t.Fatalf("last train metric: %+v", last)
	}
	ef, err := os.Open(filepath.Join(out, "chibi-eval_samples.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer ef.Close()
	samples, err := chibisensei.ParseEvalSamples(ef, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if want := min(dj.snap.Files["eval"].Rows, dj.plan.EvalSamples); len(samples) != want {
		t.Fatalf("eval samples: %d, want %d", len(samples), want)
	}
	events := chibisensei.ParseKernelLog(stdout)
	done := map[string]bool{}
	var final map[string]any
	for _, e := range events {
		if se, ok := e.Stage(); ok && se.State == "done" {
			done[se.Stage] = true
		}
		if ee, ok := e.Error(); ok {
			t.Fatalf("error event: %+v", ee)
		}
		if e.T == "done" {
			if err := json.Unmarshal(e.Raw, &final); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, s := range trainerStages {
		if !done[s] {
			t.Fatalf("stage %s has no done event", s)
		}
	}
	if final == nil || final["state"] != "succeeded" {
		t.Fatalf("done event: %v", final)
	}
	cat, err := chibisensei.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	prof, ok := cat.Get(c.profile)
	if !ok {
		t.Fatalf("profile %s missing", c.profile)
	}
	assertDryRunGGUF(t, out, prof.GGUF.ExpectedArch)
}
