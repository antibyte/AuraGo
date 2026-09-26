package llm

import (
	"encoding/csv"
	"os"
	"sort"
	"testing"
	"time"
)

// This versioned evaluation set is separate from development fixtures. Changes
// to its labels must be reviewed independently from classification rules.
func TestTaskRouterEvaluation(t *testing.T) {
	file, err := os.Open("testdata/task_router_eval.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 240 {
		t.Fatalf("evaluation size %d", len(rows))
	}
	predicted, correct, total := map[string]int{}, map[string]int{}, map[string]int{}
	local, matched, clear, edgeWrong := 0, 0, 0, 0
	durations := make([]time.Duration, 0, len(rows)*10)
	for i, row := range rows {
		if len(row) != 3 {
			t.Fatalf("row %d", i)
		}
		expected := row[1]
		if expected == "abstain" {
			expected = ""
		}
		start := time.Now()
		got := ClassifyTask(row[2]).Area()
		durations = append(durations, time.Since(start))
		if i < 180 {
			clear++
			total[expected]++
			if got != "" {
				local++
				predicted[got]++
				if got == expected {
					matched++
					correct[got]++
				}
			}
		} else if got != expected {
			edgeWrong++
			t.Logf("edge %d got %q want %q", i+1, got, expected)
		}
		if i < 180 && got != expected {
			t.Logf("clear %d got %q want %q", i+1, got, expected)
		}
	}
	for repeat := 0; repeat < 9; repeat++ {
		for _, row := range rows {
			start := time.Now()
			ClassifyTask(row[2])
			durations = append(durations, time.Since(start))
		}
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	precision := float64(matched) / float64(max(1, local))
	coverage := float64(local) / float64(clear)
	p95 := durations[len(durations)*95/100]
	t.Logf("clear precision %.1f%%, local coverage %.1f%%, fallback %.1f%%, edge errors %d/60", 100*precision, 100*coverage, 100*(1-coverage), edgeWrong)
	if p95 == 0 {
		t.Log("local p95 is below this host's clock resolution; no zero-latency claim or verified latency gate")
	} else {
		t.Logf("local classification p95 %s", p95)
	}
	for _, area := range []string{"general", "easy", "normal", "complex", "coding", "research", "creativity", "security", "writing"} {
		if total[area] != 20 {
			t.Fatalf("%s has %d clear cases", area, total[area])
		}
		t.Logf("%s precision %.1f%% recall %.1f%%", area, 100*float64(correct[area])/float64(max(1, predicted[area])), 100*float64(correct[area])/float64(total[area]))
	}
	if precision < 0.95 || coverage < 0.80 || p95 >= 5*time.Millisecond || edgeWrong > 0 {
		t.Fatal("routing quality gate failed")
	}
}
