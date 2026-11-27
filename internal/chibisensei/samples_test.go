package chibisensei

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// writeEvalFile stores lines as the run's eval_samples.jsonl below a fresh data directory.
func writeEvalFile(t *testing.T, runID string, lines ...string) string {
	t.Helper()
	dataDir := t.TempDir()
	p := filepath.Join(ChibiDir(dataDir), filepath.FromSlash(runRel(runID, outSubdir, "eval_samples.jsonl")))
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dataDir
}

func sampleLine(prompt, expected string, base, tuned *string) string {
	m := map[string]any{"expected": expected, "base_output": base, "tuned_output": tuned}
	if strings.HasPrefix(prompt, "text:") {
		m["prompt_text"] = strings.TrimPrefix(prompt, "text:")
	} else {
		m["prompt_messages"] = []Message{{Role: "user", Content: prompt}}
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func sp(s string) *string { return &s }

func indexes(p SamplePage) []int {
	out := []int{}
	for _, s := range p.Samples {
		out = append(out, s.Index)
	}
	return out
}

func eqInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestReadSamplesFiltersAndPages(t *testing.T) {
	run := Run{ID: uuid.NewString()}
	dataDir := writeEvalFile(t, run.ID,
		sampleLine("text:Q0", "A", sp("A"), sp("A")),
		sampleLine("Q1", "B", sp("x"), sp("B")),
		sampleLine("Q2", "C", sp("C"), sp("z")),
		sampleLine("Q3", "D", nil, sp("D")),
		sampleLine("Q4", "E", sp("y"), sp("y")),
	)
	read := func(recipe, kind string, labels []string, offset, limit int, filter string) SamplePage {
		t.Helper()
		p, err := ReadSamples(dataDir, run, recipe, kind, labels, offset, limit, filter)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	flag := func(b *bool) string {
		if b == nil {
			return "nil"
		}
		if *b {
			return "T"
		}
		return "F"
	}
	all := read(RecipeSpecialist, "extract", nil, 0, 20, "all")
	if all.Total != 5 || len(all.Samples) != 5 || all.Limit != 20 {
		t.Fatalf("all = %+v", all)
	}
	var base, tuned []string
	for _, s := range all.Samples {
		base, tuned = append(base, flag(s.BaseMatch)), append(tuned, flag(s.TunedMatch))
	}
	if strings.Join(base, ",") != "T,F,T,nil,F" || strings.Join(tuned, ",") != "T,T,F,T,F" {
		t.Errorf("matches base %v tuned %v", base, tuned)
	}
	if s := all.Samples[0]; s.PromptMessages != nil || s.PromptText == nil || *s.PromptText != "Q0" {
		t.Errorf("text prompt = %+v", s)
	}
	if s := all.Samples[1]; len(s.PromptMessages) != 1 || s.PromptText != nil {
		t.Errorf("message prompt = %+v", s)
	}
	if got := indexes(read(RecipeSpecialist, "extract", nil, 0, 20, "different")); !eqInts(got, []int{1, 2}) {
		t.Errorf("different = %v", got)
	}
	if p := read(RecipeSpecialist, "extract", nil, 0, 20, "wrong"); !eqInts(indexes(p), []int{2, 4}) || p.Total != 2 {
		t.Errorf("wrong = %v total %d", indexes(p), p.Total)
	}
	if p := read(RecipeSpecialist, "extract", nil, 1, 2, "all"); !eqInts(indexes(p), []int{1, 2}) || p.Total != 5 || p.Offset != 1 {
		t.Errorf("page = %v %+v", indexes(p), p)
	}
	if p := read(RecipeSpecialist, "extract", nil, 4, 2, "all"); !eqInts(indexes(p), []int{4}) {
		t.Errorf("last page = %v", indexes(p))
	}
	if p := read(RecipeSpecialist, "extract", nil, 99, 2, "all"); p.Samples == nil || len(p.Samples) != 0 || p.Total != 5 {
		t.Errorf("past the end = %+v", p)
	}
	if p := read(RecipeSpecialist, "extract", nil, -3, 0, "all"); p.Offset != 0 || p.Limit != 1 || len(p.Samples) != 1 {
		t.Errorf("clamp low = %+v", p)
	}
	if p := read(RecipeSpecialist, "extract", nil, 0, 500, "all"); p.Limit != SamplePageMax {
		t.Errorf("clamp high = %d", p.Limit)
	}
	// A free recipe has no match rule: every match is null and "wrong" is empty.
	free := read(RecipeFree, "", nil, 0, 20, "all")
	for _, s := range free.Samples {
		if s.BaseMatch != nil || s.TunedMatch != nil {
			t.Fatalf("free recipe match = %+v", s)
		}
	}
	if p := read(RecipeFree, "", nil, 0, 20, "wrong"); p.Total != 0 || p.Samples == nil {
		t.Errorf("free wrong = %+v", p)
	}
	if _, err := ReadSamples(dataDir, run, RecipeFree, "", nil, 0, 20, "bogus"); err == nil {
		t.Error("unknown filter accepted")
	}
	if _, err := ReadSamples(dataDir, Run{ID: "../x"}, RecipeFree, "", nil, 0, 20, "all"); err == nil {
		t.Error("invalid run id accepted")
	}
}

func TestReadSamplesClassifyUsesLabels(t *testing.T) {
	run := Run{ID: uuid.NewString()}
	dataDir := writeEvalFile(t, run.ID,
		sampleLine("Q0", "pos", sp("neg"), sp("Pos.")),
		sampleLine("Q1", "other", sp("neg"), sp("neg")),
	)
	p, err := ReadSamples(dataDir, run, RecipeSpecialist, "classify", []string{"pos", "neg"}, 0, 20, "all")
	if err != nil {
		t.Fatal(err)
	}
	s0, s1 := p.Samples[0], p.Samples[1]
	if s0.BaseMatch == nil || *s0.BaseMatch || s0.TunedMatch == nil || !*s0.TunedMatch {
		t.Errorf("sample 0 = %+v", s0)
	}
	if s1.BaseMatch != nil || s1.TunedMatch != nil {
		t.Errorf("an expected answer outside the labels must give null matches: %+v", s1)
	}
}

func TestReadSamplesMissingFile(t *testing.T) {
	run := Run{ID: uuid.NewString()}
	if _, err := ReadSamples(t.TempDir(), run, RecipeFree, "", nil, 0, 20, "all"); !errors.Is(err, ErrSamplesMissing) {
		t.Errorf("no chibi directory: %v", err)
	}
	dataDir := writeEvalFile(t, uuid.NewString(), sampleLine("Q", "A", nil, nil))
	if _, err := ReadSamples(dataDir, run, RecipeFree, "", nil, 0, 20, "all"); !errors.Is(err, ErrSamplesMissing) {
		t.Errorf("no file: %v", err)
	}
}
