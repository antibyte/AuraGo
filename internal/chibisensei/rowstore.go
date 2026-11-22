package chibisensei

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// AddOptions controls AddRows.
type AddOptions struct {
	Source         string      // manual|file|generated|cleaned|hf|sensei
	BatchID        string      // "" = new uuid; a running job passes the same id for every call
	BatchKind      string      // add|import|generate|clean
	MaxRows        int         // chibi_sensei.dataset.max_rows
	Partial        bool        // imports and generation: skip invalid rows and stop at MaxRows instead of refusing
	SkipDuplicates bool        // skip rows whose content_hash exists in the project or earlier in this call
	Strict         bool        // also skip rows with StructureProblems (imports, generation)
	Full           func() bool // jobs: reports that the storage limit is reached; checked before the batch, ErrStorageFull
}

// AddResult reports what AddRows did.
type AddResult struct {
	Rows           []Row `json:"rows"` // filled when <= 200 rows were added
	Added, Skipped int
	LimitReached   bool
	Issues         []Issue
	BatchID        string
	DataRev        int64
}

const maxResultIssues = 200

// rowSnapshot is every column of dataset_rows; it is the before_json of the history.
type rowSnapshot struct {
	ID           int64   `json:"id"`
	Seq          int64   `json:"seq"`
	Kind         string  `json:"kind"`
	MessagesJSON *string `json:"messages_json"`
	Text         *string `json:"text"`
	Label        string  `json:"label"`
	Split        string  `json:"split"`
	Source       string  `json:"source"`
	SourceRef    string  `json:"source_ref"`
	BatchID      string  `json:"batch_id"`
	Tags         string  `json:"tags"`
	ContentHash  string  `json:"content_hash"`
	PromptHash   string  `json:"prompt_hash"`
	EstTokens    int     `json:"est_tokens"`
	Flags        int64   `json:"flags"`
	Rev          int64   `json:"rev"`
	Created      string  `json:"created"`
	Updated      string  `json:"updated"`
}

const rowCols = `id,seq,kind,messages_json,text,label,split,source,source_ref,batch_id,tags,content_hash,prompt_hash,est_tokens,flags,rev,created,updated`

func scanSnapshot(sc rowScanner) (rowSnapshot, error) {
	var r rowSnapshot
	var mj, tx sql.NullString
	err := sc.Scan(&r.ID, &r.Seq, &r.Kind, &mj, &tx, &r.Label, &r.Split, &r.Source, &r.SourceRef, &r.BatchID, &r.Tags,
		&r.ContentHash, &r.PromptHash, &r.EstTokens, &r.Flags, &r.Rev, &r.Created, &r.Updated)
	if mj.Valid {
		r.MessagesJSON = &mj.String
	}
	if tx.Valid {
		r.Text = &tx.String
	}
	return r, err
}

func (r rowSnapshot) toRow() Row {
	row := Row{ID: r.ID, Seq: r.Seq, Kind: r.Kind, Messages: []Message{}, Text: r.Text, Label: r.Label, Split: r.Split,
		Source: r.Source, SourceRef: r.SourceRef, Tags: []string{}, EstTokens: r.EstTokens, Flags: Flag(r.Flags).Names(),
		Rev: r.Rev, BatchID: r.BatchID, ContentHash: r.ContentHash, PromptHash: r.PromptHash}
	if r.MessagesJSON != nil {
		_ = json.Unmarshal([]byte(*r.MessagesJSON), &row.Messages)
		if row.Messages == nil {
			row.Messages = []Message{}
		}
	}
	_ = json.Unmarshal([]byte(r.Tags), &row.Tags)
	if row.Tags == nil {
		row.Tags = []string{}
	}
	return row
}

// rowFields are the values derived from a RowInput.
type rowFields struct {
	kind               string
	msgsJSON           *string
	text               *string
	label, contentHash string
	promptHash         string
	est                int
	flags              Flag
}

func deriveFields(in RowInput, p Project) rowFields {
	f := rowFields{kind: rowKindChat}
	if in.Text != nil {
		f.kind = rowKindText
		t := *in.Text
		f.text = &t
		f.contentHash = ContentHash(rowKindText, nil, t)
		f.est = EstimateTokens(nil, t)
		f.flags = LocalFlags(rowKindText, nil, p.Recipe, p.TaskKind)
		return f
	}
	mj := encodeJSON(in.Messages)
	f.msgsJSON = &mj
	f.label = DeriveLabel(p.TaskKind, in.Messages)
	f.contentHash = ContentHash(rowKindChat, in.Messages, "")
	f.promptHash = PromptHash(rowKindChat, in.Messages)
	f.est = EstimateTokens(in.Messages, "")
	f.flags = LocalFlags(rowKindChat, in.Messages, p.Recipe, p.TaskKind)
	return f
}

func tagsJSON(tags []string) string {
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		out = append(out, strings.TrimSpace(t))
	}
	return encodeJSON(out)
}

func prefixIssues(prefix string, in []Issue) []Issue {
	out := make([]Issue, len(in))
	for i, x := range in {
		out[i] = x
		if x.Field != "" {
			out[i].Field = prefix + "." + x.Field
		} else {
			out[i].Field = prefix
		}
	}
	return out
}

// AddRows adds rows to a project in one transaction.
func (s *Store) AddRows(ctx context.Context, projectID string, in []RowInput, o AddOptions) (AddResult, error) {
	if o.Full != nil && o.Full() {
		return AddResult{}, ErrStorageFull
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AddResult{}, err
	}
	defer tx.Rollback()
	p, err := loadProject(ctx, tx, projectID)
	if err != nil {
		return AddResult{}, err
	}
	if o.Source == "" {
		o.Source = "manual"
	}
	callerBatch := o.BatchID != ""
	if o.BatchID == "" {
		o.BatchID = uuid.NewString()
	}
	if o.BatchKind == "" {
		o.BatchKind = "add"
	}
	openBatch := 0
	if callerBatch {
		openBatch = 1
		var undone int
		err := tx.QueryRowContext(ctx, `SELECT undone FROM batches WHERE batch_id=?`, o.BatchID).Scan(&undone)
		if err == nil && undone != 0 {
			return AddResult{}, ErrAlreadyUndone
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return AddResult{}, err
		}
	}
	res := AddResult{BatchID: o.BatchID, DataRev: p.DataRev}
	var maxSeq int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq),0) FROM dataset_rows WHERE project_id=?`, projectID).Scan(&maxSeq); err != nil {
		return AddResult{}, err
	}
	type prepared struct {
		in RowInput
		f  rowFields
	}
	var valid []prepared
	var issues []Issue
	seen := map[string]bool{}
	for i, r := range in {
		field := "rows[" + strconv.Itoa(i) + "]"
		is := ValidateRowInput(r)
		f := rowFields{}
		if len(is) == 0 {
			f = deriveFields(r, p)
			if o.Strict && f.flags&FlagStructure != 0 {
				is = []Issue{issue("", "row.structure", "The row has an invalid structure.")}
			}
		}
		if len(is) > 0 {
			if !o.Partial {
				return AddResult{}, &ValidationError{Issues: prefixIssues(field, is)}
			}
			res.Skipped++
			if len(issues) < maxResultIssues {
				issues = append(issues, prefixIssues(field, is)...)
			}
			continue
		}
		if o.SkipDuplicates {
			dup := seen[f.contentHash]
			if !dup {
				var one int
				err := tx.QueryRowContext(ctx, `SELECT 1 FROM dataset_rows WHERE project_id=? AND content_hash=? LIMIT 1`,
					projectID, f.contentHash).Scan(&one)
				if err == nil {
					dup = true
				} else if !errors.Is(err, sql.ErrNoRows) {
					return AddResult{}, err
				}
			}
			if dup {
				res.Skipped++
				continue
			}
		}
		seen[f.contentHash] = true
		valid = append(valid, prepared{r, f})
	}
	res.Issues = issues
	if o.MaxRows > 0 {
		room := o.MaxRows - p.Rows
		if room < 0 {
			room = 0
		}
		if len(valid) > room {
			if !o.Partial {
				return AddResult{}, ErrDatasetFull
			}
			valid = valid[:room]
			res.LimitReached = true
		}
	}
	if len(valid) == 0 {
		return res, nil
	}
	now := s.stamp()
	var added []Row
	for _, v := range valid {
		maxSeq++
		split := v.in.Split
		if split == "" {
			split = "auto"
		}
		tags := tagsJSON(v.in.Tags)
		r, err := tx.ExecContext(ctx, `INSERT INTO dataset_rows(project_id,seq,kind,messages_json,text,label,split,source,source_ref,batch_id,tags,
			content_hash,prompt_hash,est_tokens,flags,rev,created,updated) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,1,?,?)`,
			projectID, maxSeq, v.f.kind, v.f.msgsJSON, v.f.text, v.f.label, split, o.Source, v.in.SourceRef, o.BatchID, tags,
			v.f.contentHash, v.f.promptHash, v.f.est, int64(v.f.flags), now, now)
		if err != nil {
			return AddResult{}, err
		}
		id, _ := r.LastInsertId()
		if _, err := tx.ExecContext(ctx, `INSERT INTO row_history(project_id,row_id,batch_id,op,before_json,after_rev,created) VALUES(?,?,?,'insert',NULL,1,?)`,
			projectID, id, o.BatchID, now); err != nil {
			return AddResult{}, err
		}
		if len(valid) <= MaxRowsPerPost {
			snap := rowSnapshot{ID: id, Seq: maxSeq, Kind: v.f.kind, MessagesJSON: v.f.msgsJSON, Text: v.f.text, Label: v.f.label,
				Split: split, Source: o.Source, SourceRef: v.in.SourceRef, BatchID: o.BatchID, Tags: tags, ContentHash: v.f.contentHash,
				PromptHash: v.f.promptHash, EstTokens: v.f.est, Flags: int64(v.f.flags), Rev: 1}
			added = append(added, snap.toRow())
		}
	}
	res.Added = len(valid)
	res.Rows = added
	if _, err := tx.ExecContext(ctx, `INSERT INTO batches(batch_id,project_id,kind,row_count,undone,open,created) VALUES(?,?,?,?,0,?,?)
		ON CONFLICT(batch_id) DO UPDATE SET row_count=row_count+excluded.row_count`, o.BatchID, projectID, o.BatchKind, res.Added, openBatch, now); err != nil {
		return AddResult{}, err
	}
	res.DataRev = p.DataRev + 1
	if _, err := tx.ExecContext(ctx, `UPDATE projects SET data_rev=?, updated=? WHERE id=?`, res.DataRev, now, projectID); err != nil {
		return AddResult{}, err
	}
	if err := pruneHistory(ctx, tx, projectID); err != nil {
		return AddResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return AddResult{}, err
	}
	s.changed(projectID, res.DataRev)
	return res, nil
}

// UpdateRow replaces the content of a row when rev matches.
func (s *Store) UpdateRow(ctx context.Context, projectID string, rowID, rev int64, in RowInput) (Row, int64, error) {
	if is := ValidateRowInput(in); len(is) > 0 {
		return Row{}, 0, &ValidationError{Issues: is}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Row{}, 0, err
	}
	defer tx.Rollback()
	p, err := loadProject(ctx, tx, projectID)
	if err != nil {
		return Row{}, 0, err
	}
	old, err := scanSnapshot(tx.QueryRowContext(ctx, `SELECT `+rowCols+` FROM dataset_rows WHERE id=? AND project_id=?`, rowID, projectID))
	if errors.Is(err, sql.ErrNoRows) {
		return Row{}, 0, ErrNotFound
	}
	if err != nil {
		return Row{}, 0, err
	}
	if old.Rev != rev {
		return Row{}, 0, ErrConflict
	}
	before, _ := json.Marshal(old)
	f := deriveFields(in, p)
	next := old
	next.Kind, next.MessagesJSON, next.Text, next.Label = f.kind, f.msgsJSON, f.text, f.label
	next.ContentHash, next.PromptHash, next.EstTokens = f.contentHash, f.promptHash, f.est
	next.Flags = int64((Flag(old.Flags) &^ rowLocalFlags) | f.flags)
	if in.Split != "" {
		next.Split = in.Split
	}
	if in.Tags != nil {
		next.Tags = tagsJSON(in.Tags)
	}
	next.Rev = old.Rev + 1
	now := s.stamp()
	next.Updated = now
	batchID := uuid.NewString()
	next.BatchID = old.BatchID
	if _, err := tx.ExecContext(ctx, `UPDATE dataset_rows SET kind=?,messages_json=?,text=?,label=?,split=?,tags=?,content_hash=?,prompt_hash=?,
		est_tokens=?,flags=?,rev=?,updated=? WHERE id=?`, next.Kind, next.MessagesJSON, next.Text, next.Label, next.Split, next.Tags,
		next.ContentHash, next.PromptHash, next.EstTokens, next.Flags, next.Rev, now, rowID); err != nil {
		return Row{}, 0, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO row_history(project_id,row_id,batch_id,op,before_json,after_rev,created) VALUES(?,?,?,'update',?,?,?)`,
		projectID, rowID, batchID, string(before), next.Rev, now); err != nil {
		return Row{}, 0, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO batches(batch_id,project_id,kind,row_count,undone,created) VALUES(?,?,'edit',1,0,?)`,
		batchID, projectID, now); err != nil {
		return Row{}, 0, err
	}
	dataRev := p.DataRev + 1
	if _, err := tx.ExecContext(ctx, `UPDATE projects SET data_rev=?, updated=? WHERE id=?`, dataRev, now, projectID); err != nil {
		return Row{}, 0, err
	}
	if err := pruneHistory(ctx, tx, projectID); err != nil {
		return Row{}, 0, err
	}
	if err := tx.Commit(); err != nil {
		return Row{}, 0, err
	}
	s.changed(projectID, dataRev)
	return next.toRow(), dataRev, nil
}

// GetRow returns one row or ErrNotFound.
func (s *Store) GetRow(ctx context.Context, projectID string, rowID int64) (Row, error) {
	r, err := scanSnapshot(s.db.QueryRowContext(ctx, `SELECT `+rowCols+` FROM dataset_rows WHERE id=? AND project_id=?`, rowID, projectID))
	if errors.Is(err, sql.ErrNoRows) {
		return Row{}, ErrNotFound
	}
	if err != nil {
		return Row{}, err
	}
	return r.toRow(), nil
}

func likeEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// buildWhere builds the WHERE clause of a filter; a nil filter matches the whole project.
func buildWhere(projectID string, f *RowFilter) (string, []any, error) {
	where := []string{"project_id=?"}
	args := []any{projectID}
	if f != nil {
		if f.Q != "" {
			pat := "%" + likeEscape(f.Q) + "%"
			where = append(where, `(messages_json LIKE ? ESCAPE '\' OR text LIKE ? ESCAPE '\')`)
			args = append(args, pat, pat)
		}
		if f.Split != "" {
			where, args = append(where, "split=?"), append(args, f.Split)
		}
		if f.Source != "" {
			where, args = append(where, "source=?"), append(args, f.Source)
		}
		if f.Label != "" {
			where, args = append(where, "label=?"), append(args, f.Label)
		}
		if f.Batch != "" {
			where, args = append(where, "batch_id=?"), append(args, f.Batch)
		}
		if f.Flag != "" {
			bit, ok := ParseFlag(f.Flag)
			if !ok {
				return "", nil, &ValidationError{Issues: []Issue{issue("flag", "row.bad_flag", "Unknown flag.")}}
			}
			where, args = append(where, "(flags & ?) != 0"), append(args, int64(bit))
		}
	}
	return strings.Join(where, " AND "), args, nil
}

// ListRows returns one filtered, sorted page.
func (s *Store) ListRows(ctx context.Context, projectID string, q RowQuery) (RowPage, error) {
	if q.Limit <= 0 {
		q.Limit = PageDefault
	}
	if q.Limit > PageMax {
		q.Limit = PageMax
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	f := q.RowFilter
	where, args, err := buildWhere(projectID, &f)
	if err != nil {
		return RowPage{}, err
	}
	var dataRev int64
	if err := s.db.QueryRowContext(ctx, `SELECT data_rev FROM projects WHERE id=?`, projectID).Scan(&dataRev); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RowPage{}, ErrNotFound
		}
		return RowPage{}, err
	}
	page := RowPage{Rows: []Row{}, Offset: q.Offset, Limit: q.Limit, DataRev: dataRev}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM dataset_rows WHERE `+where, args...).Scan(&page.Total); err != nil {
		return RowPage{}, err
	}
	dir := "ASC"
	if strings.EqualFold(q.Dir, "desc") {
		dir = "DESC"
	}
	order := "seq " + dir
	if q.Sort == "est_tokens" {
		order = "est_tokens " + dir + ", seq " + dir
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+rowCols+` FROM dataset_rows WHERE `+where+` ORDER BY `+order+` LIMIT ? OFFSET ?`,
		append(args, q.Limit, q.Offset)...)
	if err != nil {
		return RowPage{}, err
	}
	defer rows.Close()
	for rows.Next() {
		r, err := scanSnapshot(rows)
		if err != nil {
			return RowPage{}, err
		}
		page.Rows = append(page.Rows, r.toRow())
	}
	return page, rows.Err()
}

func idPlaceholders(ids []int64) (string, []any) {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return strings.TrimSuffix(strings.Repeat("?,", len(ids)), ","), args
}

func (s *Store) resolveSelect(ctx context.Context, q queryer, projectID string, sel Select) ([]int64, error) {
	if len(sel.IDs) == 0 && sel.Filter == nil {
		return nil, &ValidationError{Issues: []Issue{issue("select", "row.select", "Select rows by ids or by filter.")}}
	}
	where, args, err := buildWhere(projectID, sel.Filter)
	if err != nil {
		return nil, err
	}
	scan := func(query string, a []any) ([][2]int64, error) {
		rows, err := q.QueryContext(ctx, query, a...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out [][2]int64
		for rows.Next() {
			var p [2]int64
			if err := rows.Scan(&p[0], &p[1]); err != nil {
				return nil, err
			}
			out = append(out, p)
		}
		return out, rows.Err()
	}
	var pairs [][2]int64
	if len(sel.IDs) == 0 {
		pairs, err = scan(`SELECT id, seq FROM dataset_rows WHERE `+where+` ORDER BY seq`, args)
		if err != nil {
			return nil, err
		}
	} else {
		for start := 0; start < len(sel.IDs); start += 500 {
			end := start + 500
			if end > len(sel.IDs) {
				end = len(sel.IDs)
			}
			ph, idArgs := idPlaceholders(sel.IDs[start:end])
			part, err := scan(`SELECT id, seq FROM dataset_rows WHERE `+where+` AND id IN (`+ph+`)`, append(append([]any{}, args...), idArgs...))
			if err != nil {
				return nil, err
			}
			pairs = append(pairs, part...)
		}
		sort.Slice(pairs, func(i, j int) bool { return pairs[i][1] < pairs[j][1] })
	}
	out := make([]int64, 0, len(pairs))
	var prev int64 = -1
	for _, p := range pairs {
		if p[0] != prev {
			out = append(out, p[0])
		}
		prev = p[0]
	}
	return out, nil
}

// ResolveSelect returns the ids of the selected rows in seq order.
func (s *Store) ResolveSelect(ctx context.Context, projectID string, sel Select) ([]int64, error) {
	return s.resolveSelect(ctx, s.db, projectID, sel)
}

// BulkResult reports a bulk action.
type BulkResult struct {
	Affected int    `json:"affected"`
	BatchID  string `json:"batch_id"`
	DataRev  int64  `json:"data_rev"`
}

func snapshotsByIDs(ctx context.Context, q queryer, projectID string, ids []int64) ([]rowSnapshot, error) {
	ph, args := idPlaceholders(ids)
	rows, err := q.QueryContext(ctx, `SELECT `+rowCols+` FROM dataset_rows WHERE project_id=? AND id IN (`+ph+`) ORDER BY seq`,
		append([]any{projectID}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []rowSnapshot
	for rows.Next() {
		r, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func insertHistory(ctx context.Context, tx *sql.Tx, projectID, batchID, op string, snap rowSnapshot, afterRev int64, now string) error {
	before, _ := json.Marshal(snap)
	_, err := tx.ExecContext(ctx, `INSERT INTO row_history(project_id,row_id,batch_id,op,before_json,after_rev,created) VALUES(?,?,?,?,?,?,?)`,
		projectID, snap.ID, batchID, op, string(before), afterRev, now)
	return err
}

// finishBatch records the batch, bumps data_rev, prunes the history and returns the new data_rev.
func (s *Store) finishBatch(ctx context.Context, tx *sql.Tx, p Project, batchID, kind string, affected int, now string) (int64, error) {
	if _, err := tx.ExecContext(ctx, `INSERT INTO batches(batch_id,project_id,kind,row_count,undone,created) VALUES(?,?,?,?,0,?)`,
		batchID, p.ID, kind, affected, now); err != nil {
		return 0, err
	}
	dataRev := p.DataRev + 1
	if _, err := tx.ExecContext(ctx, `UPDATE projects SET data_rev=?, updated=? WHERE id=?`, dataRev, now, p.ID); err != nil {
		return 0, err
	}
	return dataRev, pruneHistory(ctx, tx, p.ID)
}

func deleteSnapshots(ctx context.Context, tx *sql.Tx, projectID, batchID string, snaps []rowSnapshot, now string) error {
	for _, snap := range snaps {
		if err := insertHistory(ctx, tx, projectID, batchID, "delete", snap, 0, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM dataset_rows WHERE id=?`, snap.ID); err != nil {
			return err
		}
	}
	return nil
}

// Bulk applies set_split, exclude, add_tag, remove_tag or delete to the given rows.
func (s *Store) Bulk(ctx context.Context, projectID string, ids []int64, action, value string) (BulkResult, error) {
	switch action {
	case "set_split":
		if value == "" || !validSplit(value) {
			return BulkResult{}, &ValidationError{Issues: []Issue{issue("value", "row.bad_split", "Unknown split.")}}
		}
	case "exclude":
		action, value = "set_split", "exclude"
	case "add_tag", "remove_tag":
		value = strings.TrimSpace(value)
		if !validTag(value) {
			return BulkResult{}, &ValidationError{Issues: []Issue{issue("value", "row.bad_tag", "Invalid tag.")}}
		}
	case "delete":
	default:
		return BulkResult{}, &ValidationError{Issues: []Issue{issue("action", "row.bad_action", "Unknown bulk action.")}}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return BulkResult{}, err
	}
	defer tx.Rollback()
	p, err := loadProject(ctx, tx, projectID)
	if err != nil {
		return BulkResult{}, err
	}
	kind := "bulk_edit"
	if action == "delete" {
		kind = "bulk_delete"
	}
	batchID := uuid.NewString()
	now := s.stamp()
	affected := 0
	for start := 0; start < len(ids); start += 500 {
		end := start + 500
		if end > len(ids) {
			end = len(ids)
		}
		snaps, err := snapshotsByIDs(ctx, tx, projectID, ids[start:end])
		if err != nil {
			return BulkResult{}, err
		}
		if action == "delete" {
			if err := deleteSnapshots(ctx, tx, projectID, batchID, snaps, now); err != nil {
				return BulkResult{}, err
			}
			affected += len(snaps)
			continue
		}
		for _, snap := range snaps {
			split, tags := snap.Split, snap.Tags
			switch action {
			case "set_split":
				if split == value {
					continue
				}
				split = value
			case "add_tag", "remove_tag":
				var list []string
				_ = json.Unmarshal([]byte(tags), &list)
				idx := -1
				for i, t := range list {
					if t == value {
						idx = i
					}
				}
				if action == "add_tag" {
					if idx >= 0 || len(list) >= maxRowTags {
						continue
					}
					list = append(list, value)
				} else {
					if idx < 0 {
						continue
					}
					list = append(list[:idx], list[idx+1:]...)
				}
				tags = encodeJSON(list)
			}
			if err := insertHistory(ctx, tx, projectID, batchID, "update", snap, snap.Rev+1, now); err != nil {
				return BulkResult{}, err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE dataset_rows SET split=?, tags=?, rev=rev+1, updated=? WHERE id=?`, split, tags, now, snap.ID); err != nil {
				return BulkResult{}, err
			}
			affected++
		}
	}
	if affected == 0 {
		return BulkResult{DataRev: p.DataRev}, nil
	}
	dataRev, err := s.finishBatch(ctx, tx, p, batchID, kind, affected, now)
	if err != nil {
		return BulkResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return BulkResult{}, err
	}
	s.changed(projectID, dataRev)
	return BulkResult{Affected: affected, BatchID: batchID, DataRev: dataRev}, nil
}

// AnyDataRev passed to DeleteSelected skips the data_rev comparison (a delete that needs no preview token).
const AnyDataRev int64 = -1

// DeleteSelected resolves the selection and deletes the rows in one transaction. When expectedDataRev is not
// AnyDataRev and the project's data_rev differs, nothing is deleted and ErrDataChanged is returned.
func (s *Store) DeleteSelected(ctx context.Context, projectID string, sel Select, expectedDataRev int64) (BulkResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return BulkResult{}, err
	}
	defer tx.Rollback()
	p, err := loadProject(ctx, tx, projectID)
	if err != nil {
		return BulkResult{}, err
	}
	if expectedDataRev != AnyDataRev && p.DataRev != expectedDataRev {
		return BulkResult{}, ErrDataChanged
	}
	ids, err := s.resolveSelect(ctx, tx, projectID, sel)
	if err != nil {
		return BulkResult{}, err
	}
	batchID := uuid.NewString()
	now := s.stamp()
	affected := 0
	for start := 0; start < len(ids); start += 500 {
		end := start + 500
		if end > len(ids) {
			end = len(ids)
		}
		snaps, err := snapshotsByIDs(ctx, tx, projectID, ids[start:end])
		if err != nil {
			return BulkResult{}, err
		}
		if err := deleteSnapshots(ctx, tx, projectID, batchID, snaps, now); err != nil {
			return BulkResult{}, err
		}
		affected += len(snaps)
	}
	if affected == 0 {
		return BulkResult{DataRev: p.DataRev}, nil
	}
	dataRev, err := s.finishBatch(ctx, tx, p, batchID, "bulk_delete", affected, now)
	if err != nil {
		return BulkResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return BulkResult{}, err
	}
	s.changed(projectID, dataRev)
	return BulkResult{Affected: affected, BatchID: batchID, DataRev: dataRev}, nil
}

// CloseBatch marks a caller-named batch as finished, so history pruning may remove it again.
func (s *Store) CloseBatch(ctx context.Context, batchID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE batches SET open=0 WHERE batch_id=?`, batchID)
	return err
}

// EachRow calls fn for every row ordered by seq, with no open cursor across fn.
func (s *Store) EachRow(ctx context.Context, projectID string, fn func(Row) error) error {
	var last int64
	for {
		rows, err := s.db.QueryContext(ctx, `SELECT `+rowCols+` FROM dataset_rows WHERE project_id=? AND seq>? ORDER BY seq LIMIT 1000`, projectID, last)
		if err != nil {
			return err
		}
		var page []rowSnapshot
		for rows.Next() {
			r, err := scanSnapshot(rows)
			if err != nil {
				rows.Close()
				return err
			}
			page = append(page, r)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if len(page) == 0 {
			return nil
		}
		for _, r := range page {
			if err := fn(r.toRow()); err != nil {
				return err
			}
		}
		last = page[len(page)-1].Seq
	}
}

// CountRows counts the rows of a project.
func (s *Store) CountRows(ctx context.Context, projectID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM dataset_rows WHERE project_id=?`, projectID).Scan(&n)
	return n, err
}
