package chibisensei

import (
	"reflect"
	"testing"
)

func autoRows(n int) []SplitRow {
	rows := make([]SplitRow, n)
	for i := range rows {
		rows[i] = SplitRow{ID: int64(i + 1), Split: "auto", EstTokens: 100}
	}
	return rows
}

func TestSplitTargetsAndDeterminism(t *testing.T) {
	for n, want := range map[int]int{0: 0, 8: 4, 50: 5, 100: 10, 1000: 50, 5000: 200} {
		if got := TargetEval(n); got != want {
			t.Errorf("TargetEval(%d) = %d, want %d", n, got, want)
		}
	}
	for n, want := range map[int]int{199: 0, 1000: 50, 20000: 500} {
		if got := TargetVal(n); got != want {
			t.Errorf("TargetVal(%d) = %d, want %d", n, got, want)
		}
	}
	rows := autoRows(1000)
	a := ComputeSplit(rows, 7, false)
	if want := (SplitCounts{Train: 900, Val: 50, Eval: 50}); a.Counts != want {
		t.Fatalf("counts = %+v", a.Counts)
	}
	if b := ComputeSplit(rows, 7, false); !reflect.DeepEqual(a.Assign, b.Assign) {
		t.Fatal("same seed gave a different assignment")
	}
	if c := ComputeSplit(rows, 8, false); reflect.DeepEqual(a.Assign, c.Assign) {
		t.Fatal("another seed gave the same assignment")
	}

	pinned := autoRows(100)
	for i := 0; i < 3; i++ {
		pinned[i].Split = "eval"
	}
	for i := 3; i < 5; i++ {
		pinned[i].Split = "exclude"
	}
	r := ComputeSplit(pinned, 7, false)
	if r.Counts.Eval != 10 || r.Pinned.Eval != 3 || r.Counts.Excluded != 2 || r.Counts.Train+r.Counts.Val+r.Counts.Eval != 98 {
		t.Fatalf("pinned split = %+v pinned %+v", r.Counts, r.Pinned)
	}

	strat := autoRows(100)
	for i := range strat {
		strat[i].Label = "a"
		if i >= 60 {
			strat[i].Label = "b"
		}
	}
	s := ComputeSplit(strat, 3, true)
	got := map[string]int{}
	for _, row := range strat {
		if s.Assign[row.ID] == "eval" {
			got[row.Label]++
		}
	}
	if got["a"] != 6 || got["b"] != 4 {
		t.Fatalf("stratified eval = %v", got)
	}
}
