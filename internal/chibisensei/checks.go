package chibisensei

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

type TokenStats struct {
	P50 int `json:"p50"`
	P95 int `json:"p95"`
	Max int `json:"max"`
}

type CheckSummary struct {
	Rows      int        `json:"rows"`
	Train     int        `json:"train"`
	Val       int        `json:"val"`
	Eval      int        `json:"eval"`
	EstTokens TokenStats `json:"est_tokens"`
}

type Check struct {
	ID         string         `json:"id"`
	Level      string         `json:"level"` // ok|info|warn|block
	Value      float64        `json:"value"`
	Threshold  float64        `json:"threshold"`
	MessageKey string         `json:"message_key"`
	Args       map[string]any `json:"args"`
	RowIDs     []int64        `json:"row_ids"`
	RowCount   int            `json:"row_count"`
	Fix        string         `json:"fix,omitempty"`
}

type CheckResult struct {
	ProjectID  string       `json:"project_id"`
	DataRev    int64        `json:"data_rev"`
	ComputedAt string       `json:"computed_at"`
	Summary    CheckSummary `json:"summary"`
	Checks     []Check      `json:"checks"`
	Ready      bool         `json:"ready"`
}

type CheckRow struct {
	ID, Seq                 int64
	Kind                    string
	Messages                []Message
	Text, Label, Split      string
	ContentHash, PromptHash string
	EstTokens               int
}

type CheckInput struct {
	Project   Project
	Profile   *Profile
	Rows      []CheckRow
	Split     SplitResult
	MaxSeqLen int
}

const (
	checkPrefix     = "chibi_sensei.check."
	maxCheckRowIDs  = 50
	freeMinTokens   = 20000
	minTrainedEvals = 10
)

func recipeCounts(recipe string) (min, rec int) {
	switch recipe {
	case RecipeSpecialist:
		return 30, 100
	case RecipeFree:
		return 20, 200
	}
	return 30, 200
}

// checkBuilder collects the ids of one check in seq order.
type checkBuilder struct{ ids []int64 }

func (b *checkBuilder) add(id int64) { b.ids = append(b.ids, id) }

func (b *checkBuilder) fill(c Check) Check {
	c.RowCount = len(b.ids)
	c.RowIDs = b.ids
	if len(c.RowIDs) > maxCheckRowIDs {
		c.RowIDs = c.RowIDs[:maxCheckRowIDs]
	}
	if c.RowIDs == nil {
		c.RowIDs = []int64{}
	}
	if c.Args == nil {
		c.Args = map[string]any{}
	}
	return c
}

func okCheck(id string, value, threshold float64) Check {
	return Check{ID: id, Level: "ok", Value: value, Threshold: threshold, MessageKey: checkPrefix + "ok", Args: map[string]any{}, RowIDs: []int64{}}
}

func tokenStats(rows []CheckRow) TokenStats {
	est := make([]int, 0, len(rows))
	maxV := 0
	for _, r := range rows {
		est = append(est, r.EstTokens)
		if r.EstTokens > maxV {
			maxV = r.EstTokens
		}
	}
	return TokenStats{P50: Percentile(est, 0.5), P95: Percentile(est, 0.95), Max: maxV}
}

// RunChecks returns the result and the flags per row (local and cross-row). Excluded rows are ignored except by
// structure. row_ids hold at most 50 IDs in seq order; row_count holds all. Checks that do not apply are left out.
func RunChecks(in CheckInput, now time.Time) (CheckResult, map[int64]Flag) {
	p := in.Project
	rows := append([]CheckRow(nil), in.Rows...)
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Seq < rows[j].Seq })

	flags := map[int64]Flag{}
	var active []CheckRow // non-excluded
	for _, r := range rows {
		if f := LocalFlags(r.Kind, r.Messages, p.Recipe, p.TaskKind); f != 0 {
			flags[r.ID] = f
		}
		if r.Split != "exclude" {
			active = append(active, r)
		}
	}
	n := len(active)
	var sumEst int
	for _, r := range active {
		sumEst += r.EstTokens
	}
	minRows, recRows := recipeCounts(p.Recipe)
	var checks []Check

	// count_min
	{
		c := Check{ID: "count_min", Value: float64(n), Threshold: float64(recRows),
			Args: map[string]any{"count": n, "min": minRows, "recommended": recRows, "tokens": sumEst}}
		switch {
		case n < minRows || (p.Recipe == RecipeFree && sumEst < freeMinTokens):
			c.Level, c.Threshold, c.MessageKey = "block", float64(minRows), checkPrefix+"count_min"
		case n < recRows:
			c.Level, c.MessageKey = "warn", checkPrefix+"count_min_recommended"
		default:
			c.Level, c.MessageKey = "ok", checkPrefix+"ok"
		}
		c.RowIDs = []int64{}
		checks = append(checks, c)
	}

	// structure: every row, including excluded ones
	{
		var b checkBuilder
		for _, r := range rows {
			if flags[r.ID]&FlagStructure != 0 {
				b.add(r.ID)
			}
		}
		if len(b.ids) > 0 {
			checks = append(checks, b.fill(Check{ID: "structure", Level: "block", Value: float64(len(b.ids)), MessageKey: checkPrefix + "structure"}))
		} else {
			checks = append(checks, okCheck("structure", 0, 0))
		}
	}

	// short_answer
	if p.Recipe == RecipeAssistant || p.Recipe == RecipeSpecialist {
		var b checkBuilder
		for _, r := range active {
			if flags[r.ID]&FlagShortAnswer != 0 {
				b.add(r.ID)
			}
		}
		minChars := 10
		if p.Recipe == RecipeSpecialist {
			minChars = 2
		}
		if len(b.ids) > 0 {
			checks = append(checks, b.fill(Check{ID: "short_answer", Level: "warn", Value: float64(len(b.ids)), Threshold: float64(minChars),
				MessageKey: checkPrefix + "short_answer", Args: map[string]any{"min_chars": minChars}}))
		} else {
			checks = append(checks, okCheck("short_answer", 0, float64(minChars)))
		}
	}

	// duplicates
	{
		var b checkBuilder
		seen := map[string]bool{}
		for _, r := range active {
			if seen[r.ContentHash] {
				b.add(r.ID)
				flags[r.ID] |= FlagDuplicate
			}
			seen[r.ContentHash] = true
		}
		if len(b.ids) > 0 {
			checks = append(checks, b.fill(Check{ID: "duplicates", Level: "warn", Value: float64(len(b.ids)),
				MessageKey: checkPrefix + "duplicates", Args: map[string]any{"count": len(b.ids)}, Fix: "dedupe"}))
		} else {
			checks = append(checks, okCheck("duplicates", 0, 0))
		}
	}

	// conflicts
	if p.Recipe == RecipeSpecialist {
		groups := map[string][]CheckRow{}
		for _, r := range active {
			if r.PromptHash != "" {
				groups[r.PromptHash] = append(groups[r.PromptHash], r)
			}
		}
		var b checkBuilder
		conflicting := map[int64]bool{}
		nGroups := 0
		for _, g := range groups {
			answers := map[string]bool{}
			for _, r := range g {
				answers[normalizeForHash(LastAssistant(r.Messages))] = true
			}
			if len(answers) > 1 {
				nGroups++
				for _, r := range g {
					conflicting[r.ID] = true
				}
			}
		}
		for _, r := range active {
			if conflicting[r.ID] {
				b.add(r.ID)
				flags[r.ID] |= FlagConflict
			}
		}
		if len(b.ids) > 0 {
			checks = append(checks, b.fill(Check{ID: "conflicts", Level: "warn", Value: float64(nGroups),
				MessageKey: checkPrefix + "conflicts", Args: map[string]any{"groups": nGroups, "count": len(b.ids)}}))
		} else {
			checks = append(checks, okCheck("conflicts", 0, 0))
		}
	}

	// too_long
	if in.MaxSeqLen > 0 {
		var b checkBuilder
		for _, r := range active {
			if r.EstTokens > in.MaxSeqLen {
				b.add(r.ID)
				flags[r.ID] |= FlagTooLong
			}
		}
		if len(b.ids) > 0 {
			share := float64(len(b.ids)) / float64(n)
			key := checkPrefix + "too_long"
			if share >= 0.20 {
				key = checkPrefix + "too_long_strong"
			}
			checks = append(checks, b.fill(Check{ID: "too_long", Level: "warn", Value: share, Threshold: 0.20, MessageKey: key,
				Args: map[string]any{"count": len(b.ids), "share_pct": int(share*100 + 0.5), "max_seq_len": in.MaxSeqLen}}))
		} else {
			checks = append(checks, okCheck("too_long", 0, 0.20))
		}
	}

	// label_balance
	if p.TaskKind == "classify" {
		counts := map[string]int{}
		for _, r := range active {
			if r.Label != "" {
				counts[r.Label]++
			}
		}
		switch {
		case len(counts) == 1:
			var b checkBuilder
			for _, r := range active {
				if r.Label != "" {
					b.add(r.ID)
				}
			}
			var only string
			for l := range counts {
				only = l
			}
			checks = append(checks, b.fill(Check{ID: "label_balance", Level: "block", Value: 1, Threshold: 2,
				MessageKey: checkPrefix + "label_single", Args: map[string]any{"labels": map[string]int{only: counts[only]}}}))
		case len(counts) > 1:
			minC, maxC := -1, 0
			for _, c := range counts {
				if minC < 0 || c < minC {
					minC = c
				}
				if c > maxC {
					maxC = c
				}
			}
			ratio := float64(maxC) / float64(minC)
			rare := map[string]bool{}
			for l, c := range counts {
				if c < 10 || c*5 < maxC {
					rare[l] = true
				}
			}
			if len(rare) > 0 || ratio > 5 {
				var b checkBuilder
				for _, r := range active {
					if rare[r.Label] {
						b.add(r.ID)
						flags[r.ID] |= FlagRareLabel
					}
				}
				checks = append(checks, b.fill(Check{ID: "label_balance", Level: "warn", Value: ratio, Threshold: 5,
					MessageKey: checkPrefix + "label_balance", Args: map[string]any{"labels": counts, "min_count": minC, "ratio": ratio}}))
			} else {
				checks = append(checks, okCheck("label_balance", ratio, 5))
			}
		}
	}

	// json_valid
	if p.TaskKind == "to_json" {
		var b checkBuilder
		for _, r := range active {
			if flags[r.ID]&FlagInvalidJSON != 0 {
				b.add(r.ID)
			}
		}
		if len(b.ids) > 0 {
			checks = append(checks, b.fill(Check{ID: "json_valid", Level: "block", Value: float64(len(b.ids)),
				MessageKey: checkPrefix + "json_valid"}))
		} else {
			checks = append(checks, okCheck("json_valid", 0, 0))
		}
	}

	// split_sizes
	{
		sc := in.Split.Counts
		c := Check{ID: "split_sizes", Args: map[string]any{"train": sc.Train, "val": sc.Val, "eval": sc.Eval}, RowIDs: []int64{}}
		switch {
		case sc.Train < minRows:
			c.Level, c.Value, c.Threshold, c.MessageKey = "block", float64(sc.Train), float64(minRows), checkPrefix+"split_train_small"
		case sc.Eval < minTrainedEvals:
			c.Level, c.Value, c.Threshold, c.MessageKey = "warn", float64(sc.Eval), minTrainedEvals, checkPrefix+"split_eval_small"
		default:
			c.Level, c.Value, c.Threshold, c.MessageKey = "ok", float64(sc.Eval), minTrainedEvals, checkPrefix+"ok"
		}
		checks = append(checks, c)
	}

	// language_fit
	if in.Profile != nil && !containsString(in.Profile.Arch.Languages, "*") {
		sample := make([]Row, 0, len(active))
		for _, r := range active {
			row := Row{Seq: r.Seq, Messages: r.Messages}
			if r.Kind == rowKindText {
				t := r.Text
				row.Text = &t
			}
			sample = append(sample, row)
		}
		if lang := DetectLanguage(LanguageSample(p.GoalText, sample)); lang != "" {
			if containsString(in.Profile.Arch.Languages, lang) {
				checks = append(checks, okCheck("language_fit", 0, 0))
			} else {
				c := okCheck("language_fit", 0, 0)
				c.Level, c.MessageKey, c.Args = "info", checkPrefix+"language_fit", map[string]any{"language": lang}
				checks = append(checks, c)
			}
		}
	}

	ready := true
	for _, c := range checks {
		if c.Level == "block" {
			ready = false
		}
	}
	res := CheckResult{ProjectID: p.ID, DataRev: p.DataRev, ComputedAt: now.UTC().Format(time.RFC3339),
		Summary: CheckSummary{Rows: n, Train: in.Split.Counts.Train, Val: in.Split.Counts.Val, Eval: in.Split.Counts.Eval,
			EstTokens: tokenStats(active)},
		Checks: checks, Ready: ready}
	return res, flags
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// LoadCheckRows reads the project and its rows in one read transaction, paged by seq.
func (s *Store) LoadCheckRows(ctx context.Context, projectID string) ([]CheckRow, Project, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, Project{}, err
	}
	defer tx.Rollback()
	p, err := loadProject(ctx, tx, projectID)
	if err != nil {
		return nil, Project{}, err
	}
	out := []CheckRow{}
	var last int64
	for {
		rows, err := tx.QueryContext(ctx, `SELECT id, seq, kind, messages_json, text, label, split, content_hash, prompt_hash, est_tokens
			FROM dataset_rows WHERE project_id=? AND seq>? ORDER BY seq LIMIT 1000`, projectID, last)
		if err != nil {
			return nil, Project{}, err
		}
		n := 0
		for rows.Next() {
			var r CheckRow
			var mj, text sql.NullString
			if err := rows.Scan(&r.ID, &r.Seq, &r.Kind, &mj, &text, &r.Label, &r.Split, &r.ContentHash, &r.PromptHash, &r.EstTokens); err != nil {
				rows.Close()
				return nil, Project{}, err
			}
			if mj.Valid {
				_ = json.Unmarshal([]byte(mj.String), &r.Messages)
			}
			if r.Messages == nil {
				r.Messages = []Message{}
			}
			r.Text = text.String
			last = r.Seq
			n++
			out = append(out, r)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, Project{}, err
		}
		if n == 0 {
			return out, p, nil
		}
	}
}

// SaveChecks stores a check run: ErrDataChanged when data_rev moved; otherwise in one transaction the flags of the
// project are reset, the given flags set and the result upserted. It does not bump data_rev or row revs.
func (s *Store) SaveChecks(ctx context.Context, res CheckResult, flags map[int64]Flag) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	p, err := loadProject(ctx, tx, res.ProjectID)
	if err != nil {
		return err
	}
	if p.DataRev != res.DataRev {
		return ErrDataChanged
	}
	if _, err := tx.ExecContext(ctx, `UPDATE dataset_rows SET flags=0 WHERE project_id=?`, res.ProjectID); err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `UPDATE dataset_rows SET flags=? WHERE id=? AND project_id=?`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for id, f := range flags {
		if f == 0 {
			continue
		}
		if _, err := stmt.ExecContext(ctx, int64(f), id, res.ProjectID); err != nil {
			return err
		}
	}
	ready := 0
	if res.Ready {
		ready = 1
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO checks(project_id,data_rev,ready,result_json,computed_at) VALUES(?,?,?,?,?)
		ON CONFLICT(project_id) DO UPDATE SET data_rev=excluded.data_rev, ready=excluded.ready,
		result_json=excluded.result_json, computed_at=excluded.computed_at`,
		res.ProjectID, res.DataRev, ready, encodeJSON(res), res.ComputedAt); err != nil {
		return err
	}
	return tx.Commit()
}

// GetChecks returns the stored result (nil when none) and whether it is stale (data_rev moved since).
func (s *Store) GetChecks(ctx context.Context, projectID string) (*CheckResult, bool, error) {
	p, err := s.GetProject(ctx, projectID)
	if err != nil {
		return nil, false, err
	}
	var raw string
	err = s.db.QueryRowContext(ctx, `SELECT result_json FROM checks WHERE project_id=?`, projectID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var res CheckResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &res); err != nil {
		return nil, false, err
	}
	return &res, res.DataRev != p.DataRev, nil
}
