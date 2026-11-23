package chibisensei

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
)

var snapOpts = AddOptions{Source: "manual", BatchKind: "add", MaxRows: 50000}

func TestRun12AndNames(t *testing.T) {
	id := "0123abcd-4567-4890-abcd-ef0123456789"
	if got := Run12(id); got != "0123abcd4567" {
		t.Fatalf("Run12 = %q", got)
	}
	pattern := regexp.MustCompile(`^r[0-9a-f]{12}-(train|val|eval)\.jsonl$|^r[0-9a-f]{12}-marker\.json$`)
	for _, split := range SnapshotSplits {
		if name := SnapshotFileName(id, split); !pattern.MatchString(name) || name != "r0123abcd4567-"+split+".jsonl" {
			t.Fatalf("SnapshotFileName(%s) = %q", split, name)
		}
	}
	if got := MarkerFileName(id); got != "r0123abcd4567-marker.json" || !pattern.MatchString(got) {
		t.Fatalf("MarkerFileName = %q", got)
	}
	if got := DatasetSlug(id); got != "chibi-data-0123abcd4567" {
		t.Fatalf("DatasetSlug = %q", got)
	}
	if got := NotebookSlug("1a2b3c4d-0000-4000-8000-000000000000", id); got != "chibi-1a2b3c4d-0123abcd4567" {
		t.Fatalf("NotebookSlug = %q", got)
	}
	for recipe, want := range map[string]string{RecipeAssistant: "messages", RecipeSpecialist: "messages", RecipeFree: "text", "x": ""} {
		if got := FormatFor(recipe); got != want {
			t.Errorf("FormatFor(%q) = %q, want %q", recipe, got, want)
		}
	}
}

func snapProject(t *testing.T, st *Store, recipe string, n int, excludeAt int) string {
	t.Helper()
	ctx := context.Background()
	p, err := st.CreateProject(ctx, ProjectInput{Name: "Snap", Recipe: recipe})
	if err != nil {
		t.Fatal(err)
	}
	in := make([]RowInput, 0, n)
	for i := 0; i < n; i++ {
		var r RowInput
		if recipe == RecipeFree {
			text := fmt.Sprintf("free text number %d is a sentence long enough to count", i)
			r = RowInput{Text: &text}
		} else if i == excludeAt {
			r = chatInput("excluded-marker-zzz", "this answer must stay out of every file")
		} else {
			r = chatInput(fmt.Sprintf("question %d", i), fmt.Sprintf("answer number %d is long enough", i))
		}
		if i == excludeAt {
			r.Split = "exclude"
		}
		in = append(in, r)
	}
	if _, err := st.AddRows(ctx, p.ID, in, snapOpts); err != nil {
		t.Fatal(err)
	}
	return p.ID
}

func TestFreezeSnapshotAssistant(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	pid := snapProject(t, st, RecipeAssistant, 240, 7)
	runID := uuid.NewString()
	dir := t.TempDir()
	snap, err := st.FreezeSnapshot(ctx, pid, runID, dir)
	if err != nil {
		t.Fatal(err)
	}
	proj, _ := st.GetProject(ctx, pid)
	srows, _, _ := st.LoadSplitRows(ctx, pid)
	sp := ComputeSplit(srows, proj.SplitSeed, false)
	want := map[string]int{"train": sp.Counts.Train, "val": sp.Counts.Val, "eval": sp.Counts.Eval}
	if sp.Counts.Val == 0 || sp.Counts.Eval == 0 {
		t.Fatalf("setup should give val and eval rows: %+v", sp.Counts)
	}
	total, size := 0, int64(0)
	for _, split := range SnapshotSplits {
		f, ok := snap.Files[split]
		if !ok || f.Rows != want[split] || f.Name != SnapshotFileName(runID, split) {
			t.Fatalf("file %s = %+v (ok=%v), want %d rows", split, f, ok, want[split])
		}
		data, err := os.ReadFile(filepath.Join(dir, f.Name))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != f.SHA256 || int64(len(data)) != f.Bytes {
			t.Fatalf("%s hash or size differs from the file", split)
		}
		if bytes.Contains(data, []byte("excluded-marker-zzz")) {
			t.Fatalf("excluded row leaked into %s", split)
		}
		total += f.Rows
		size += f.Bytes
	}
	if total != 239 {
		t.Fatalf("rows in files = %d, want 239", total)
	}
	mdata, err := os.ReadFile(filepath.Join(dir, snap.Marker.Name))
	if err != nil {
		t.Fatal(err)
	}
	var m snapshotMarker
	if err := json.Unmarshal(mdata, &m); err != nil {
		t.Fatal(err)
	}
	if m.V != 1 || m.RunID != runID || m.Format != "messages" || m.DataRev != proj.DataRev || len(m.Files) != 3 {
		t.Fatalf("marker = %+v", m)
	}
	for k, f := range snap.Files {
		if m.Files[k] != f {
			t.Fatalf("marker file %s = %+v, want %+v", k, m.Files[k], f)
		}
	}
	if snap.DataSHA != snapshotDataSHA(snap.Files) || snap.Format != "messages" || snap.DataRev != proj.DataRev {
		t.Fatalf("snapshot facts = %+v", snap)
	}
	if snap.Bytes != size+int64(len(mdata)) {
		t.Fatalf("Bytes = %d, want %d", snap.Bytes, size+int64(len(mdata)))
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 4 {
		t.Fatalf("directory has %d entries, want 4", len(entries))
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".part") {
			t.Fatalf("left over %s", e.Name())
		}
	}

	dir2 := t.TempDir()
	snap2, err := st.FreezeSnapshot(ctx, pid, runID, dir2)
	if err != nil {
		t.Fatal(err)
	}
	if snap2.DataSHA != snap.DataSHA || snap2.Marker.SHA256 != snap.Marker.SHA256 {
		t.Fatal("a second freeze must be deterministic")
	}
	for _, split := range SnapshotSplits {
		a, _ := os.ReadFile(filepath.Join(dir, snap.Files[split].Name))
		b, _ := os.ReadFile(filepath.Join(dir2, snap2.Files[split].Name))
		if !bytes.Equal(a, b) {
			t.Fatalf("%s differs between freezes", split)
		}
	}
}

func TestFreezeSnapshotNoVal(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	pid := snapProject(t, st, RecipeAssistant, 60, -1)
	runID := uuid.NewString()
	dir := t.TempDir()
	snap, err := st.FreezeSnapshot(ctx, pid, runID, dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := snap.Files["val"]; ok {
		t.Fatalf("val must be absent: %+v", snap.Files)
	}
	if _, err := os.Stat(filepath.Join(dir, SnapshotFileName(runID, "val"))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("val file exists or unexpected error: %v", err)
	}
	mdata, _ := os.ReadFile(filepath.Join(dir, snap.Marker.Name))
	var raw struct {
		Files map[string]json.RawMessage `json:"files"`
	}
	if err := json.Unmarshal(mdata, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw.Files["val"]; ok || raw.Files["train"] == nil {
		t.Fatalf("marker files = %v", raw.Files)
	}
}

func TestFreezeSnapshotFreeText(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	pid := snapProject(t, st, RecipeFree, 30, -1)
	dir := t.TempDir()
	snap, err := st.FreezeSnapshot(ctx, pid, uuid.NewString(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Format != "text" {
		t.Fatalf("format = %q", snap.Format)
	}
	for split, f := range snap.Files {
		data, err := os.ReadFile(filepath.Join(dir, f.Name))
		if err != nil {
			t.Fatal(err)
		}
		lines := bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n"))
		if len(lines) != f.Rows {
			t.Fatalf("%s has %d lines, want %d", split, len(lines), f.Rows)
		}
		for _, l := range lines {
			var v map[string]json.RawMessage
			if err := json.Unmarshal(l, &v); err != nil || len(v) != 1 || v["text"] == nil {
				t.Fatalf("%s line %q is not {\"text\":...}", split, l)
			}
		}
	}
}

func TestFreezeSnapshotDataChanged(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	pid := snapProject(t, st, RecipeAssistant, 60, -1)
	freezeAfterLoad = func() {
		if _, err := st.AddRows(ctx, pid, []RowInput{chatInput("late", "late answer arrives here")}, snapOpts); err != nil {
			t.Errorf("late add: %v", err)
		}
	}
	t.Cleanup(func() { freezeAfterLoad = nil })
	dir := t.TempDir()
	_, err := st.FreezeSnapshot(ctx, pid, uuid.NewString(), dir)
	if !errors.Is(err, ErrDataChanged) {
		t.Fatalf("err = %v, want ErrDataChanged", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("directory not empty: %v", entries)
	}
}

func TestFreezeSnapshotRejects(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	pid := snapProject(t, st, RecipeAssistant, 5, -1)
	if _, err := st.FreezeSnapshot(ctx, pid, "not-a-uuid", t.TempDir()); err == nil {
		t.Fatal("invalid run id accepted")
	}
	ep, err := st.CreateProject(ctx, ProjectInput{Name: "Excluded", Recipe: RecipeAssistant})
	if err != nil {
		t.Fatal(err)
	}
	ex := ep.ID
	for i := 0; i < 3; i++ {
		r := chatInput(fmt.Sprintf("question %d", i), fmt.Sprintf("answer number %d is long enough", i))
		r.Split = "exclude"
		if _, err := st.AddRows(ctx, ex, []RowInput{r}, snapOpts); err != nil {
			t.Fatal(err)
		}
	}
	_, err = st.FreezeSnapshot(ctx, ex, uuid.NewString(), t.TempDir())
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want *ValidationError", err)
	}
}
