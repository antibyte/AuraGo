package chibisensei

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/chibisensei/trainer"
)

var updateGolden = flag.Bool("update", false, "rewrite the golden files and the proof data under testdata")

const goldenRunID = "0123abcd-4567-4890-abcd-ef0123456789"

// goldenLock is fixed so pin changes at the proof runs do not churn the golden files.
var goldenLock = trainer.Lock{Pip: []string{"peft==0.21.3"}, LlamaCppTag: "b9994",
	LlamaCppURL: trainer.LlamaCppURLPrefix + "b9994"}

type goldenCase struct {
	profile, recipe, taskKind string
	labels                    []string
}

var goldenCases = []goldenCase{
	{"gemma3-270m", RecipeSpecialist, "classify", []string{"account", "billing", "delivery", "technical"}},
	{"qwen35-0p8b", RecipeAssistant, "", nil},
	{"lfm25-1p2b", RecipeFree, "", nil},
}

func goldenInput(t *testing.T, c *Catalog, gc goldenCase, lock trainer.Lock) JobInput {
	t.Helper()
	prof, ok := c.Get(gc.profile)
	if !ok {
		t.Fatalf("profile %s missing", gc.profile)
	}
	rows := make([]SplitRow, 412)
	assign := map[int64]string{}
	for i := range rows {
		rows[i] = SplitRow{ID: int64(i + 1), Split: "auto", EstTokens: 180}
		switch {
		case i < 371:
			assign[rows[i].ID] = "train"
		case i < 392:
			assign[rows[i].ID] = "val"
		default:
			assign[rows[i].ID] = "eval"
		}
	}
	labels := gc.labels
	if labels == nil {
		labels = []string{}
	}
	proj := Project{ProjectSummary: ProjectSummary{ID: "11111111-2222-4333-8444-555555555555", Recipe: gc.recipe,
		TaskKind: gc.taskKind, ProfileID: gc.profile, DataRev: 7}, Labels: labels, Preset: "balanced"}
	plan, err := ComputePlan(PlanInput{Project: proj, Profile: prof, Rows: rows,
		Split:                SplitResult{Counts: SplitCounts{Train: 371, Val: 21, Eval: 20}, Assign: assign},
		RemoteTimeoutMinutes: 60, NeedBaseGGUF: true})
	if err != nil {
		t.Fatal(err)
	}
	return JobInput{RunID: goldenRunID, Created: time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC),
		AuraGoVersion: "test", Project: proj, Profile: prof, ProfileSHA: c.SHA(gc.profile), Plan: plan,
		Marker: MarkerFileName(goldenRunID), Format: FormatFor(gc.recipe), Lock: lock}
}

func goldenJob(t *testing.T, c *Catalog, gc goldenCase, lock trainer.Lock) JobSpec {
	t.Helper()
	spec, err := BuildJobSpec(goldenInput(t, c, gc, lock))
	if err != nil {
		t.Fatal(err)
	}
	return spec
}

func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name)
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v (create it with -update)", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs; review the change, then run go test ./internal/chibisensei -run '(?i)golden' -update", path)
	}
}

func TestJobSpecGolden(t *testing.T) {
	c := mustCatalog(t)
	for _, gc := range goldenCases {
		a, err := goldenJob(t, c, gc, goldenLock).JSON()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := goldenJob(t, c, gc, goldenLock).JSON()
		if !bytes.Equal(a, b) {
			t.Fatalf("%s: job.json is not deterministic", gc.profile)
		}
		checkGolden(t, "job-"+gc.profile+".json", a)
	}
}

func TestNotebookGolden(t *testing.T) {
	job, err := goldenJob(t, mustCatalog(t), goldenCases[1], goldenLock).JSON()
	if err != nil {
		t.Fatal(err)
	}
	stub := []byte("print(\"chibi stub\")\n")
	nb, err := BuildNotebook(goldenRunID, job, stub)
	if err != nil {
		t.Fatal(err)
	}
	nb2, _ := BuildNotebook(goldenRunID, job, stub)
	if !bytes.Equal(nb, nb2) {
		t.Fatal("notebook is not deterministic")
	}
	checkGolden(t, "notebook.ipynb", nb)
}
