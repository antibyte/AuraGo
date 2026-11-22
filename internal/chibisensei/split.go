package chibisensei

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"sort"
)

// SplitRow is the part of a row the split needs.
type SplitRow struct {
	ID        int64
	Split     string
	Label     string
	EstTokens int
}

type SplitCounts struct {
	Train    int `json:"train"`
	Val      int `json:"val"`
	Eval     int `json:"eval"`
	Excluded int `json:"excluded"`
}

type SplitPinned struct {
	Train int `json:"train"`
	Eval  int `json:"eval"`
}

type SplitResult struct {
	Seed   int64            `json:"seed"`
	Counts SplitCounts      `json:"counts"`
	Pinned SplitPinned      `json:"pinned"`
	Assign map[int64]string `json:"-"` // row id -> train|val|eval|exclude
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// TargetEval: n == 0 gives 0; n >= 100 gives clamp(round(0.05n), 10, 200); otherwise max(5, round(0.1n)); then at most n/2.
func TargetEval(n int) int {
	if n <= 0 {
		return 0
	}
	var t int
	if n >= 100 {
		t = clampInt(int(math.Round(0.05*float64(n))), 10, 200)
	} else {
		t = int(math.Round(0.1 * float64(n)))
		if t < 5 {
			t = 5
		}
	}
	if t > n/2 {
		t = n / 2
	}
	return t
}

// TargetVal: n < 200 gives 0; otherwise clamp(round(0.05n), 10, 500).
func TargetVal(n int) int {
	if n < 200 {
		return 0
	}
	return clampInt(int(math.Round(0.05*float64(n))), 10, 500)
}

type keyedRow struct {
	row SplitRow
	key [sha256.Size]byte
}

func splitKey(seed, id int64) [sha256.Size]byte {
	var b [16]byte
	binary.BigEndian.PutUint64(b[:8], uint64(seed))
	binary.BigEndian.PutUint64(b[8:], uint64(id))
	return sha256.Sum256(b[:])
}

// distribute splits quota over the groups by largest remainder: floor(quota*n_g/total), remainders descending,
// ties by label ascending. A group never receives more than it holds.
func distribute(quota int, labels []string, counts map[string]int, total int) map[string]int {
	out := map[string]int{}
	if quota <= 0 || total <= 0 {
		return out
	}
	type rem struct {
		label string
		r     int
	}
	var rems []rem
	given := 0
	for _, l := range labels {
		q := quota * counts[l] / total
		out[l] = q
		given += q
		rems = append(rems, rem{l, quota * counts[l] % total})
	}
	sort.SliceStable(rems, func(i, j int) bool {
		if rems[i].r != rems[j].r {
			return rems[i].r > rems[j].r
		}
		return rems[i].label < rems[j].label
	})
	for i := 0; given < quota && i < len(rems); i++ {
		if out[rems[i].label] < counts[rems[i].label] {
			out[rems[i].label]++
			given++
		}
	}
	return out
}

// ComputeSplit assigns every row to train, val, eval or exclude. See the plan for the exact rules.
func ComputeSplit(rows []SplitRow, seed int64, stratify bool) SplitResult {
	res := SplitResult{Seed: seed, Assign: make(map[int64]string, len(rows))}
	var auto []keyedRow
	for _, r := range rows {
		switch r.Split {
		case "exclude":
			res.Counts.Excluded++
			res.Assign[r.ID] = "exclude"
		case "train":
			res.Pinned.Train++
			res.Assign[r.ID] = "train"
		case "eval":
			res.Pinned.Eval++
			res.Assign[r.ID] = "eval"
		default:
			auto = append(auto, keyedRow{row: r, key: splitKey(seed, r.ID)})
		}
	}
	n := len(rows) - res.Counts.Excluded
	sort.SliceStable(auto, func(i, j int) bool { return bytes.Compare(auto[i].key[:], auto[j].key[:]) < 0 })
	evalQ := TargetEval(n) - res.Pinned.Eval
	if evalQ < 0 {
		evalQ = 0
	}
	if evalQ > len(auto) {
		evalQ = len(auto)
	}
	valQ := TargetVal(n)
	if valQ > len(auto)-evalQ {
		valQ = len(auto) - evalQ
	}
	if !stratify {
		for i, k := range auto {
			switch {
			case i < evalQ:
				res.Assign[k.row.ID] = "eval"
			case i < evalQ+valQ:
				res.Assign[k.row.ID] = "val"
			default:
				res.Assign[k.row.ID] = "train"
			}
		}
	} else {
		byLabel := map[string][]keyedRow{}
		for _, k := range auto {
			byLabel[k.row.Label] = append(byLabel[k.row.Label], k)
		}
		labels := make([]string, 0, len(byLabel))
		counts := map[string]int{}
		for l, list := range byLabel {
			labels = append(labels, l)
			counts[l] = len(list)
		}
		sort.Strings(labels)
		evalPer := distribute(evalQ, labels, counts, len(auto))
		remaining := map[string]int{}
		for _, l := range labels {
			remaining[l] = counts[l] - evalPer[l]
		}
		valPer := distribute(valQ, labels, remaining, len(auto)-evalQ)
		for _, l := range labels {
			for i, k := range byLabel[l] {
				switch {
				case i < evalPer[l]:
					res.Assign[k.row.ID] = "eval"
				case i < evalPer[l]+valPer[l]:
					res.Assign[k.row.ID] = "val"
				default:
					res.Assign[k.row.ID] = "train"
				}
			}
		}
	}
	for _, r := range rows {
		switch res.Assign[r.ID] {
		case "train":
			res.Counts.Train++
		case "val":
			res.Counts.Val++
		case "eval":
			res.Counts.Eval++
		}
	}
	return res
}

// LoadSplitRows reads the project and its rows in one read transaction, paged by seq.
func (s *Store) LoadSplitRows(ctx context.Context, projectID string) ([]SplitRow, Project, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, Project{}, err
	}
	defer tx.Rollback()
	p, err := loadProject(ctx, tx, projectID)
	if err != nil {
		return nil, Project{}, err
	}
	out := []SplitRow{}
	var last int64
	for {
		rows, err := tx.QueryContext(ctx, `SELECT id, seq, split, label, est_tokens FROM dataset_rows
			WHERE project_id=? AND seq>? ORDER BY seq LIMIT 1000`, projectID, last)
		if err != nil {
			return nil, Project{}, err
		}
		n := 0
		for rows.Next() {
			var r SplitRow
			var seq int64
			if err := rows.Scan(&r.ID, &seq, &r.Split, &r.Label, &r.EstTokens); err != nil {
				rows.Close()
				return nil, Project{}, err
			}
			last = seq
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

// Reseed stores a new random seed and bumps data_rev. It writes no history.
func (s *Store) Reseed(ctx context.Context, projectID string) (Project, error) {
	seed, err := randomSeed()
	if err != nil {
		return Project{}, err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE projects SET split_seed=?, data_rev=data_rev+1, updated=? WHERE id=?`,
		seed, s.stamp(), projectID)
	if err != nil {
		return Project{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Project{}, ErrNotFound
	}
	p, err := s.GetProject(ctx, projectID)
	if err != nil {
		return Project{}, err
	}
	s.changed(projectID, p.DataRev)
	return p, nil
}
