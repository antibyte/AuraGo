package chibisensei

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

var checkNow = time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)

func chatCheckRow(id int64, user, answer string, est int) CheckRow {
	msgs := []Message{{"user", user}, {"assistant", answer}}
	return CheckRow{ID: id, Seq: id, Kind: rowKindChat, Messages: msgs, Label: answer, Split: "auto",
		ContentHash: ContentHash(rowKindChat, msgs, ""), PromptHash: PromptHash(rowKindChat, msgs), EstTokens: est}
}

func uniqueRows(n int, answer string, est int) []CheckRow {
	rows := make([]CheckRow, n)
	for i := range rows {
		rows[i] = chatCheckRow(int64(i+1), fmt.Sprintf("question number %d", i), answer, est)
	}
	return rows
}

func runCheck(t *testing.T, in CheckInput) (CheckResult, map[int64]Flag) {
	t.Helper()
	if in.Project.Recipe == "" {
		in.Project.Recipe = RecipeAssistant
	}
	if in.Split.Counts.Train == 0 {
		in.Split.Counts = SplitCounts{Train: len(in.Rows), Eval: 20}
	}
	return RunChecks(in, checkNow)
}

func findCheck(res CheckResult, id string) *Check {
	for i := range res.Checks {
		if res.Checks[i].ID == id {
			return &res.Checks[i]
		}
	}
	return nil
}

func TestChecksFireOnFixtures(t *testing.T) {
	longAnswer := "a sufficiently long answer"
	qwen, lfm, gemma := (*Profile)(nil), (*Profile)(nil), (*Profile)(nil)
	c := mustCatalog(t)
	qwen, _ = c.Get("qwen35-0p8b")
	lfm, _ = c.Get("lfm25-1p2b")
	gemma, _ = c.Get("gemma3-270m")
	_ = qwen

	t.Run("count_min", func(t *testing.T) {
		for n, want := range map[int]string{10: "block", 50: "warn", 250: "ok"} {
			res, _ := runCheck(t, CheckInput{Rows: uniqueRows(n, longAnswer, 50)})
			if got := findCheck(res, "count_min"); got == nil || got.Level != want {
				t.Errorf("%d rows: %+v, want %s", n, got, want)
			}
		}
		res, _ := runCheck(t, CheckInput{Project: Project{ProjectSummary: ProjectSummary{Recipe: RecipeFree}}, Rows: uniqueRows(25, longAnswer, 100)})
		if got := findCheck(res, "count_min"); got.Level != "block" || got.Args["tokens"] != 2500 {
			t.Errorf("free rows: %+v", got)
		}
		if res.Ready {
			t.Error("a block check must clear Ready")
		}
	})
	t.Run("structure", func(t *testing.T) {
		rows := uniqueRows(40, longAnswer, 50)
		rows[6].Messages = []Message{{"user", "only a question"}}
		res, flags := runCheck(t, CheckInput{Rows: rows})
		got := findCheck(res, "structure")
		if got.Level != "block" || !reflect.DeepEqual(got.RowIDs, []int64{rows[6].ID}) || flags[rows[6].ID]&FlagStructure == 0 {
			t.Errorf("structure: %+v", got)
		}
	})
	t.Run("short_answer", func(t *testing.T) {
		rows := uniqueRows(40, longAnswer, 50)
		rows[0].Messages[1].Content = "ok"
		res, _ := runCheck(t, CheckInput{Rows: rows})
		if got := findCheck(res, "short_answer"); got.Level != "warn" || got.RowCount != 1 {
			t.Errorf("assistant: %+v", got)
		}
		res, _ = runCheck(t, CheckInput{Project: Project{ProjectSummary: ProjectSummary{Recipe: RecipeSpecialist, TaskKind: "rewrite"}}, Rows: rows})
		if got := findCheck(res, "short_answer"); got.Level != "ok" {
			t.Errorf("specialist: %+v", got)
		}
	})
	t.Run("duplicates", func(t *testing.T) {
		rows := uniqueRows(40, longAnswer, 50)
		rows[1], rows[2], rows[3] = chatCheckRow(2, "same", longAnswer, 50), chatCheckRow(3, "same", longAnswer, 50), chatCheckRow(4, "same", longAnswer, 50)
		res, flags := runCheck(t, CheckInput{Rows: rows})
		got := findCheck(res, "duplicates")
		if got.Level != "warn" || got.Value != 2 || got.Fix != "dedupe" || flags[3]&FlagDuplicate == 0 || flags[4]&FlagDuplicate == 0 || flags[2]&FlagDuplicate != 0 {
			t.Errorf("duplicates: %+v flags %v", got, flags)
		}
	})
	t.Run("conflicts", func(t *testing.T) {
		rows := uniqueRows(40, longAnswer, 50)
		rows[0], rows[1] = chatCheckRow(1, "same prompt", "a", 50), chatCheckRow(2, "same prompt", "b", 50)
		res, flags := runCheck(t, CheckInput{Project: Project{ProjectSummary: ProjectSummary{Recipe: RecipeSpecialist, TaskKind: "rewrite"}}, Rows: rows})
		got := findCheck(res, "conflicts")
		if got.Level != "warn" || got.RowCount != 2 || flags[1]&FlagConflict == 0 {
			t.Errorf("conflicts: %+v", got)
		}
	})
	t.Run("too_long", func(t *testing.T) {
		rows := uniqueRows(40, longAnswer, 50)
		for i := 0; i < 10; i++ {
			rows[i].EstTokens = 300
		}
		res, flags := runCheck(t, CheckInput{Rows: rows, MaxSeqLen: 256})
		got := findCheck(res, "too_long")
		if got.Level != "warn" || got.Args["share_pct"] != 25 || got.MessageKey != "chibi_sensei.check.too_long_strong" || flags[1]&FlagTooLong == 0 {
			t.Errorf("too_long: %+v", got)
		}
	})
	t.Run("label_balance", func(t *testing.T) {
		cls := Project{ProjectSummary: ProjectSummary{Recipe: RecipeSpecialist, TaskKind: "classify"}}
		res, _ := runCheck(t, CheckInput{Project: cls, Rows: uniqueRows(40, "pos", 20)})
		if got := findCheck(res, "label_balance"); got.Level != "block" || got.MessageKey != "chibi_sensei.check.label_single" {
			t.Errorf("single: %+v", got)
		}
		rows := uniqueRows(55, "pos", 20)
		for i := 50; i < 55; i++ {
			rows[i] = chatCheckRow(int64(i+1), fmt.Sprintf("q%d", i), "neg", 20)
		}
		res, flags := runCheck(t, CheckInput{Project: cls, Rows: rows})
		if got := findCheck(res, "label_balance"); got.Level != "warn" || got.RowCount != 5 || flags[51]&FlagRareLabel == 0 || flags[1]&FlagRareLabel != 0 {
			t.Errorf("rare: %+v", got)
		}
	})
	t.Run("json_valid", func(t *testing.T) {
		rows := uniqueRows(40, `{"a":1}`, 20)
		rows[4].Messages[1].Content = "{x"
		res, _ := runCheck(t, CheckInput{Project: Project{ProjectSummary: ProjectSummary{Recipe: RecipeSpecialist, TaskKind: "to_json"}}, Rows: rows})
		if got := findCheck(res, "json_valid"); got.Level != "block" || !reflect.DeepEqual(got.RowIDs, []int64{5}) {
			t.Errorf("json_valid: %+v", got)
		}
	})
	t.Run("split_sizes", func(t *testing.T) {
		res, _ := RunChecks(CheckInput{Project: Project{ProjectSummary: ProjectSummary{Recipe: RecipeAssistant}}, Rows: uniqueRows(40, longAnswer, 50),
			Split: SplitResult{Counts: SplitCounts{Train: 20, Eval: 5}}}, checkNow)
		if got := findCheck(res, "split_sizes"); got.Level != "block" || got.MessageKey != "chibi_sensei.check.split_train_small" {
			t.Errorf("split_sizes: %+v", got)
		}
	})
	t.Run("language_fit", func(t *testing.T) {
		rows := make([]CheckRow, 40)
		for i := range rows {
			rows[i] = chatCheckRow(int64(i+1), fmt.Sprintf("Привет, как дела у тебя сегодня %d", i), "Хорошо, спасибо, а у тебя как дела сегодня", 30)
		}
		res, _ := runCheck(t, CheckInput{Profile: lfm, Rows: rows})
		if got := findCheck(res, "language_fit"); got == nil || got.Level != "info" || got.Args["language"] != "ru" {
			t.Errorf("lfm: %+v", got)
		}
		res, _ = runCheck(t, CheckInput{Profile: gemma, Rows: rows})
		if findCheck(res, "language_fit") != nil {
			t.Error("gemma must not get a language check")
		}
	})
	t.Run("persist", func(t *testing.T) {
		ctx := context.Background()
		st := newTestStore(t)
		p, err := st.CreateProject(ctx, ProjectInput{Name: "P", Recipe: RecipeAssistant})
		if err != nil {
			t.Fatal(err)
		}
		in := []RowInput{chatInput("q one", "a long enough answer"), chatInput("q two", "ok")}
		if _, err := st.AddRows(ctx, p.ID, in, AddOptions{Source: "manual", BatchKind: "add", MaxRows: 100}); err != nil {
			t.Fatal(err)
		}
		rows, proj, err := st.LoadCheckRows(ctx, p.ID)
		if err != nil || len(rows) != 2 {
			t.Fatalf("load: %v %d", err, len(rows))
		}
		res, flags := RunChecks(CheckInput{Project: proj, Rows: rows, Split: SplitResult{Counts: SplitCounts{Train: 2}}}, checkNow)
		if err := st.SaveChecks(ctx, res, flags); err != nil {
			t.Fatal(err)
		}
		got, stale, err := st.GetChecks(ctx, p.ID)
		if err != nil || got == nil || stale || got.DataRev != res.DataRev || len(got.Checks) != len(res.Checks) {
			t.Fatalf("get: %+v stale=%v err=%v", got, stale, err)
		}
		if _, err := st.AddRows(ctx, p.ID, []RowInput{chatInput("q three", "another long answer")}, AddOptions{Source: "manual", BatchKind: "add", MaxRows: 100}); err != nil {
			t.Fatal(err)
		}
		if _, stale, _ := st.GetChecks(ctx, p.ID); !stale {
			t.Error("result must be stale after a data change")
		}
		if err := st.SaveChecks(ctx, res, flags); !errors.Is(err, ErrDataChanged) {
			t.Errorf("stale save: %v", err)
		}
		if !strings.HasPrefix(res.Checks[0].MessageKey, checkPrefix) {
			t.Error("bad key prefix")
		}
	})
}
