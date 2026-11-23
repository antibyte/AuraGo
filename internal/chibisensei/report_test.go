package chibisensei

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func fp(v float64) *float64 { return &v }

func testStatus(base, tuned *float64) Status {
	var st Status
	st.Train.EvalLossBase, st.Train.EvalLossTuned = base, tuned
	return st
}

func samples(rows ...[3]string) []EvalSample {
	out := make([]EvalSample, 0, len(rows))
	for _, r := range rows {
		r := r
		out = append(out, EvalSample{Expected: r[0], BaseOutput: &r[1], TunedOutput: &r[2]})
	}
	return out
}

func TestScoreReportClassify(t *testing.T) {
	rep := ScoreReport(RecipeSpecialist, "classify", []string{"pos", "neg"}, testStatus(fp(2.31), fp(1.12)), samples(
		[3]string{"pos", "pos", "pos"}, [3]string{"neg", "pos", "neg"},
		[3]string{"pos", "Positive", "pos."}, [3]string{"neg", "neg", "NEG"}))
	la := rep.LabelAccuracy
	if la == nil || la.N != 4 || la.Base != 0.5 || la.Tuned != 1 || rep.ExactMatch != nil || rep.Perplexity != nil {
		t.Fatalf("report: %+v", rep)
	}
	if !reflect.DeepEqual(la.Confusion.Base, [][]int{{1, 0, 1}, {1, 1, 0}}) ||
		!reflect.DeepEqual(la.Confusion.Tuned, [][]int{{2, 0, 0}, {0, 2, 0}}) {
		t.Fatalf("confusion: %+v", la.Confusion)
	}
	if !rep.SmallSample || rep.Samples != 4 || *rep.Loss.Base != 2.31 {
		t.Fatalf("meta: %+v", rep)
	}
	if _, err := json.Marshal(rep); err != nil {
		t.Fatal(err)
	}
}

func TestScoreReportCases(t *testing.T) {
	st := testStatus(fp(2), fp(1))

	t.Run("assistant", func(t *testing.T) {
		rep := ScoreReport(RecipeAssistant, "", nil, st, samples([3]string{"a", "a", "a"}, [3]string{"b", "b", "b"}, [3]string{"c", "c", "c"}))
		if rep.Loss == nil || rep.ExactMatch != nil || rep.LabelAccuracy != nil || rep.Perplexity != nil || !rep.SmallSample {
			t.Fatalf("report: %+v", rep)
		}
	})
	t.Run("extract", func(t *testing.T) {
		rep := ScoreReport(RecipeSpecialist, "extract", nil, st, samples([3]string{"Berlin", " berlin", "Berlin  "}, [3]string{"Paris", "Paris", "Paris"}))
		em := rep.ExactMatch
		if em == nil || em.Base != 0.5 || em.Tuned != 1 || em.N != 2 || em.NBase != nil {
			t.Fatalf("exact match: %+v", em)
		}
	})
	t.Run("rewrite NFC", func(t *testing.T) {
		rep := ScoreReport(RecipeSpecialist, "rewrite", nil, st, samples([3]string{"Café", "x", "Cafe" + string(rune(0x301))}))
		if em := rep.ExactMatch; em == nil || em.Tuned != 1 || em.Base != 0 {
			t.Fatalf("exact match: %+v", em)
		}
	})
	t.Run("to_json", func(t *testing.T) {
		rep := ScoreReport(RecipeSpecialist, "to_json", nil, st, samples(
			[3]string{`{"a":1,"b":[1,2]}`, `{"a":1}`, `{ "b": [1, 2], "a": 1.0 }`},
			[3]string{"plain", "not json", " plain "}))
		if em := rep.ExactMatch; em == nil || em.Base != 0 || em.Tuned != 1 || em.N != 2 {
			t.Fatalf("exact match: %+v", em)
		}
	})
	t.Run("free", func(t *testing.T) {
		rep := ScoreReport(RecipeFree, "", nil, testStatus(fp(2), fp(1)), nil)
		if rep.Perplexity == nil || *rep.Perplexity.Base != 7.389 || *rep.Perplexity.Tuned != 2.718 || rep.ExactMatch != nil || rep.LabelAccuracy != nil {
			t.Fatalf("report: %+v", rep)
		}
	})
	t.Run("free nil tuned", func(t *testing.T) {
		rep := ScoreReport(RecipeFree, "", nil, testStatus(fp(2), nil), nil)
		if rep.Perplexity == nil || rep.Perplexity.Tuned != nil || rep.Loss == nil || rep.Loss.Tuned != nil {
			t.Fatalf("report: %+v", rep)
		}
	})
	t.Run("no losses", func(t *testing.T) {
		if rep := ScoreReport(RecipeFree, "", nil, testStatus(nil, nil), nil); rep.Loss != nil || rep.Perplexity != nil {
			t.Fatalf("report: %+v", rep)
		}
	})
	t.Run("perplexity overflow", func(t *testing.T) {
		if rep := ScoreReport(RecipeFree, "", nil, testStatus(fp(1000), fp(1000)), nil); rep.Perplexity != nil || rep.Loss == nil {
			t.Fatalf("report: %+v", rep)
		}
	})
	t.Run("30 samples", func(t *testing.T) {
		rows := make([][3]string, 30)
		for i := range rows {
			rows[i] = [3]string{"a", "a", "a"}
		}
		if rep := ScoreReport(RecipeAssistant, "", nil, st, samples(rows...)); rep.SmallSample || rep.Samples != 30 {
			t.Fatalf("report: %+v", rep)
		}
	})
	t.Run("classify without labels", func(t *testing.T) {
		if rep := ScoreReport(RecipeSpecialist, "classify", nil, st, samples([3]string{"a", "a", "a"})); rep.LabelAccuracy != nil {
			t.Fatalf("report: %+v", rep)
		}
	})
	t.Run("expected is no label", func(t *testing.T) {
		rep := ScoreReport(RecipeSpecialist, "classify", []string{"pos", "neg"}, st, samples([3]string{"pos", "pos", "pos"}, [3]string{"???", "pos", "pos"}))
		if la := rep.LabelAccuracy; la == nil || la.N != 1 || la.NBase != nil {
			t.Fatalf("label accuracy: %+v", la)
		}
	})
}

func TestScoreReportNilAnswers(t *testing.T) {
	st := testStatus(fp(2), fp(1))
	s := func(v string) *string { return &v }
	t.Run("exact match", func(t *testing.T) {
		rep := ScoreReport(RecipeSpecialist, "extract", nil, st, []EvalSample{
			{Expected: "a", BaseOutput: s("a"), TunedOutput: s("a")},
			{Expected: "b", BaseOutput: nil, TunedOutput: s("b")},
			{Expected: "c", BaseOutput: s("x"), TunedOutput: nil},
			{Expected: "d", BaseOutput: s(""), TunedOutput: s("d")},
		})
		em := rep.ExactMatch
		// base scores a, c, d (1 correct); tuned scores a, b, d (all correct); equal counts omit n_base
		if em == nil || em.N != 3 || em.NBase != nil || em.Tuned != 1 || em.Base != 0.3333 {
			t.Fatalf("exact match: %+v", em)
		}
		rep = ScoreReport(RecipeSpecialist, "extract", nil, st, []EvalSample{
			{Expected: "a", BaseOutput: nil, TunedOutput: s("a")},
			{Expected: "b", BaseOutput: nil, TunedOutput: s("x")},
			{Expected: "c", BaseOutput: s("c"), TunedOutput: s("c")},
		})
		em = rep.ExactMatch
		if em == nil || em.N != 3 || em.NBase == nil || *em.NBase != 1 || em.Base != 1 {
			t.Fatalf("exact match: %+v", em)
		}
		raw, _ := json.Marshal(rep.ExactMatch)
		if !strings.Contains(string(raw), `"n_base":1`) {
			t.Fatalf("json: %s", raw)
		}
	})
	t.Run("nothing generated", func(t *testing.T) {
		rep := ScoreReport(RecipeSpecialist, "extract", nil, st, []EvalSample{{Expected: "a"}})
		if rep.ExactMatch != nil || rep.Samples != 1 {
			t.Fatalf("report: %+v", rep)
		}
	})
	t.Run("classify", func(t *testing.T) {
		rep := ScoreReport(RecipeSpecialist, "classify", []string{"pos", "neg"}, st, []EvalSample{
			{Expected: "pos", BaseOutput: s("neg"), TunedOutput: s("pos")},
			{Expected: "neg", BaseOutput: nil, TunedOutput: s("neg")},
		})
		la := rep.LabelAccuracy
		if la == nil || la.N != 2 || la.NBase == nil || *la.NBase != 1 || la.Base != 0 || la.Tuned != 1 {
			t.Fatalf("label accuracy: %+v", la)
		}
		if !reflect.DeepEqual(la.Confusion.Base, [][]int{{0, 1, 0}, {0, 0, 0}}) ||
			!reflect.DeepEqual(la.Confusion.Tuned, [][]int{{1, 0, 0}, {0, 1, 0}}) {
			t.Fatalf("confusion: %+v", la.Confusion)
		}
	})
}
