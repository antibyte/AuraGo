package chibisensei

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"math/big"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// embeddedCatalog is the profile catalog, loaded once; UpdateProject validates overrides against it.
var embeddedCatalog = sync.OnceValues(LoadCatalog)

var profileIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,40}$`)

// queryer is satisfied by *sql.DB and *sql.Tx.
type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func (s *Store) stamp() string { return s.now().UTC().Format(time.RFC3339) }

func (s *Store) changed(projectID string, dataRev int64) {
	if s.notify != nil {
		s.notify(projectID, dataRev)
	}
}

func validTaskKind(k string) bool {
	switch k {
	case "classify", "extract", "rewrite", "to_json":
		return true
	}
	return false
}

func validRecipe(r string) bool {
	for _, x := range Recipes() {
		if x.ID == r {
			return true
		}
	}
	return false
}

// normalizeLabels cleans the labels; non-classify projects get []. Violations are reported as issues.
func normalizeLabels(taskKind string, labels []string) ([]string, []Issue) {
	if taskKind != "classify" {
		return []string{}, nil
	}
	out := make([]string, 0, len(labels))
	seen := map[string]bool{}
	var issues []Issue
	if len(labels) > 50 {
		issues = append(issues, issue("labels", "project.labels", "A classifier accepts at most 50 labels."))
	}
	for _, l := range labels {
		l = strings.TrimSpace(l)
		n := utf8.RuneCountInString(l)
		if n < 1 || n > 64 {
			issues = append(issues, issue("labels", "project.labels", "A label needs 1 to 64 characters."))
			continue
		}
		if seen[l] {
			issues = append(issues, issue("labels", "project.labels", "Labels must be unique."))
			continue
		}
		seen[l] = true
		out = append(out, l)
	}
	return out, issues
}

// validateProject checks the fields of a project and returns the cleaned labels.
func validateProject(p *Project) []Issue {
	var issues []Issue
	p.Name = strings.TrimSpace(p.Name)
	if n := utf8.RuneCountInString(p.Name); n < 1 || n > 80 {
		issues = append(issues, issue("name", "project.name", "The name needs 1 to 80 characters."))
	}
	if !validRecipe(p.Recipe) {
		issues = append(issues, issue("recipe", "project.recipe", "Unknown recipe."))
	}
	if p.Recipe == RecipeSpecialist {
		if !validTaskKind(p.TaskKind) {
			issues = append(issues, issue("task_kind", "project.task_kind", "A specialist needs a task kind."))
		}
	} else if p.TaskKind != "" {
		issues = append(issues, issue("task_kind", "project.task_kind", "Only a specialist has a task kind."))
	}
	labels, li := normalizeLabels(p.TaskKind, p.Labels)
	p.Labels = labels
	issues = append(issues, li...)
	if utf8.RuneCountInString(p.GoalText) > 4000 {
		issues = append(issues, issue("goal_text", "project.goal_text", "The goal text is too long."))
	}
	if p.ProfileID != "" && !profileIDPattern.MatchString(p.ProfileID) {
		issues = append(issues, issue("profile_id", "project.profile_id", "Invalid profile id."))
	}
	switch p.Preset {
	case "quick", "balanced", "thorough":
	default:
		issues = append(issues, issue("preset", "project.preset", "Unknown preset."))
	}
	return issues
}

func randomSeed() (int64, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1<<31-1))
	if err != nil {
		return 0, err
	}
	return n.Int64() + 1, nil // 1..2^31-1
}

// CreateProject validates the input and inserts a project (rev 1, data_rev 0, preset balanced).
func (s *Store) CreateProject(ctx context.Context, in ProjectInput) (Project, error) {
	var p Project
	p.Name, p.Recipe, p.TaskKind = in.Name, in.Recipe, in.TaskKind
	p.Labels, p.GoalText, p.ProfileID = in.Labels, in.GoalText, in.ProfileID
	p.Preset = "balanced"
	if issues := validateProject(&p); len(issues) > 0 {
		return Project{}, &ValidationError{Issues: issues}
	}
	seed, err := randomSeed()
	if err != nil {
		return Project{}, err
	}
	now := s.stamp()
	p.ID, p.SplitSeed, p.Rev, p.DataRev = uuid.NewString(), seed, 1, 0
	p.Created, p.Updated = now, now
	_, err = s.db.ExecContext(ctx, `INSERT INTO projects(id,name,recipe,task_kind,labels_json,goal_text,profile_id,preset,overrides_json,
		split_seed,data_rev,rev,created,updated,archived) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,0)`,
		p.ID, p.Name, p.Recipe, p.TaskKind, encodeJSON(p.Labels), p.GoalText, p.ProfileID, p.Preset, encodeJSON(p.Overrides),
		p.SplitSeed, p.DataRev, p.Rev, p.Created, p.Updated)
	if err != nil {
		return Project{}, err
	}
	p.Steps = computeSteps(p, 0, sql.NullInt64{}, false, projectRuns{})
	return p, nil
}

const projectCols = `p.id,p.name,p.recipe,p.task_kind,p.labels_json,p.goal_text,p.profile_id,p.preset,p.overrides_json,
	p.split_seed,p.data_rev,p.rev,p.created,p.updated,p.archived`

type rowScanner interface{ Scan(dest ...any) error }

// scanProject scans projectCols plus the row count and the check state, and computes the steps.
func scanProject(sc rowScanner) (Project, error) {
	var p Project
	var labels, overrides string
	var archived int
	var rows int
	var checkRev sql.NullInt64
	var ready sql.NullInt64
	var latest sql.NullString
	var succeeded int
	err := sc.Scan(&p.ID, &p.Name, &p.Recipe, &p.TaskKind, &labels, &p.GoalText, &p.ProfileID, &p.Preset, &overrides,
		&p.SplitSeed, &p.DataRev, &p.Rev, &p.Created, &p.Updated, &archived, &rows, &checkRev, &ready, &latest, &succeeded)
	if err != nil {
		return Project{}, err
	}
	p.Labels = []string{}
	_ = json.Unmarshal([]byte(labels), &p.Labels)
	if p.Labels == nil {
		p.Labels = []string{}
	}
	_ = json.Unmarshal([]byte(overrides), &p.Overrides)
	p.Archived = archived != 0
	p.Rows = rows
	p.Steps = computeSteps(p, rows, checkRev, ready.Valid && ready.Int64 != 0,
		projectRuns{Latest: latest.String, Succeeded: succeeded != 0})
	return p, nil
}

const projectSelect = `SELECT ` + projectCols + `, (SELECT COUNT(*) FROM dataset_rows r WHERE r.project_id=p.id), c.data_rev, c.ready,
	(SELECT x.state FROM runs x WHERE x.project_id=p.id ORDER BY x.created DESC, x.rowid DESC LIMIT 1),
	EXISTS(SELECT 1 FROM runs x WHERE x.project_id=p.id AND x.state='succeeded')
	FROM projects p LEFT JOIN checks c ON c.project_id=p.id`

func loadProject(ctx context.Context, q queryer, id string) (Project, error) {
	p, err := scanProject(q.QueryRowContext(ctx, projectSelect+` WHERE p.id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	return p, err
}

// GetProject returns one project or ErrNotFound.
func (s *Store) GetProject(ctx context.Context, id string) (Project, error) {
	return loadProject(ctx, s.db, id)
}

// ListProjects lists the active or the archived projects, newest updated first.
func (s *Store) ListProjects(ctx context.Context, archived bool) ([]ProjectSummary, error) {
	flag := 0
	if archived {
		flag = 1
	}
	rows, err := s.db.QueryContext(ctx, projectSelect+` WHERE p.archived=? ORDER BY p.updated DESC, p.rowid DESC`, flag)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProjectSummary{}
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p.ProjectSummary)
	}
	return out, rows.Err()
}

// projectRuns are the run facts the training step needs: the state of the newest run and whether any succeeded.
type projectRuns struct {
	Latest    string
	Succeeded bool
}

func computeSteps(p Project, rows int, checkRev sql.NullInt64, checkReady bool, runs projectRuns) Steps {
	st := Steps{Goal: "in_progress", Data: "todo", Training: "todo", Test: "todo", Use: "todo"}
	if p.GoalText != "" && p.ProfileID != "" && (p.Recipe != RecipeSpecialist || p.TaskKind != "") &&
		(p.TaskKind != "classify" || len(p.Labels) >= 2) {
		st.Goal = "done"
	}
	switch {
	case rows == 0:
	case checkRev.Valid && checkRev.Int64 == p.DataRev && checkReady:
		st.Data = "ready"
	case checkRev.Valid && checkRev.Int64 == p.DataRev:
		st.Data = "blocked"
	default:
		st.Data = "in_progress"
	}
	// Precedence: an active run, then any success, then a failed or cancelled latest run, then the data state.
	switch {
	case runs.Latest != "" && !RunTerminal(runs.Latest):
		st.Training = "in_progress"
	case runs.Succeeded:
		st.Training = "done"
	case runs.Latest == RunFailed || runs.Latest == RunCancelled:
		st.Training = "blocked"
	case st.Data == "ready":
		st.Training = "ready"
	}
	return st
}

// UpdateProject applies the non-nil fields when rev matches (ErrConflict otherwise) and increments rev.
func (s *Store) UpdateProject(ctx context.Context, id string, rev int64, patch ProjectPatch) (Project, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Project{}, err
	}
	defer tx.Rollback()
	cur, err := loadProject(ctx, tx, id)
	if err != nil {
		return Project{}, err
	}
	if cur.Rev != rev {
		return Project{}, ErrConflict
	}
	next := cur
	if patch.Name != nil {
		next.Name = *patch.Name
	}
	if patch.Recipe != nil {
		next.Recipe = *patch.Recipe
	}
	if patch.TaskKind != nil {
		next.TaskKind = *patch.TaskKind
	}
	if patch.GoalText != nil {
		next.GoalText = *patch.GoalText
	}
	if patch.ProfileID != nil {
		next.ProfileID = *patch.ProfileID
	}
	if patch.Preset != nil {
		next.Preset = *patch.Preset
	}
	if patch.Labels != nil {
		next.Labels = *patch.Labels
	}
	if patch.Overrides != nil {
		next.Overrides = *patch.Overrides
	}
	if issues := validateProject(&next); len(issues) > 0 {
		return Project{}, &ValidationError{Issues: issues}
	}
	if next.ProfileID != cur.ProfileID || next.Preset != cur.Preset || next.Overrides != cur.Overrides {
		if next.ProfileID != "" {
			cat, err := embeddedCatalog()
			if err != nil {
				return Project{}, err
			}
			if prof, ok := cat.Get(next.ProfileID); ok {
				if issues := ValidateOverrides(next.Overrides, prof); len(issues) > 0 {
					return Project{}, &ValidationError{Issues: issues}
				}
			}
		}
	}
	dataChanged := next.Recipe != cur.Recipe || next.TaskKind != cur.TaskKind || next.ProfileID != cur.ProfileID ||
		next.Preset != cur.Preset || next.Overrides != cur.Overrides || encodeJSON(next.Labels) != encodeJSON(cur.Labels)
	if next.Recipe != cur.Recipe || next.TaskKind != cur.TaskKind {
		if err := s.recomputeRows(ctx, tx, next); err != nil {
			return Project{}, err
		}
	}
	next.Rev = cur.Rev + 1
	if dataChanged {
		next.DataRev = cur.DataRev + 1
	}
	next.Updated = s.stamp()
	if _, err := tx.ExecContext(ctx, `UPDATE projects SET name=?,recipe=?,task_kind=?,labels_json=?,goal_text=?,profile_id=?,preset=?,
		overrides_json=?,data_rev=?,rev=?,updated=? WHERE id=?`, next.Name, next.Recipe, next.TaskKind, encodeJSON(next.Labels),
		next.GoalText, next.ProfileID, next.Preset, encodeJSON(next.Overrides), next.DataRev, next.Rev, next.Updated, id); err != nil {
		return Project{}, err
	}
	out, err := loadProject(ctx, tx, id)
	if err != nil {
		return Project{}, err
	}
	if err := tx.Commit(); err != nil {
		return Project{}, err
	}
	if dataChanged {
		s.changed(id, out.DataRev)
	}
	return out, nil
}

// recomputeRows refreshes label and the local flags of every row, paged by seq.
func (s *Store) recomputeRows(ctx context.Context, tx *sql.Tx, p Project) error {
	var last int64
	for {
		type item struct {
			id, flags int64
			kind      string
			msgsJSON  sql.NullString
		}
		rows, err := tx.QueryContext(ctx, `SELECT id, seq, kind, messages_json, flags FROM dataset_rows
			WHERE project_id=? AND seq>? ORDER BY seq LIMIT 1000`, p.ID, last)
		if err != nil {
			return err
		}
		var page []item
		for rows.Next() {
			var it item
			var seq int64
			if err := rows.Scan(&it.id, &seq, &it.kind, &it.msgsJSON, &it.flags); err != nil {
				rows.Close()
				return err
			}
			last = seq
			page = append(page, it)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		if len(page) == 0 {
			return nil
		}
		for _, it := range page {
			var msgs []Message
			if it.msgsJSON.Valid {
				_ = json.Unmarshal([]byte(it.msgsJSON.String), &msgs)
			}
			flags := (Flag(it.flags) &^ rowLocalFlags) | LocalFlags(it.kind, msgs, p.Recipe, p.TaskKind)
			if _, err := tx.ExecContext(ctx, `UPDATE dataset_rows SET label=?, flags=? WHERE id=?`,
				DeriveLabel(p.TaskKind, msgs), int64(flags), it.id); err != nil {
				return err
			}
		}
	}
}

// SetArchived archives or restores a project and increments rev.
func (s *Store) SetArchived(ctx context.Context, id string, archived bool) (Project, error) {
	flag := 0
	if archived {
		flag = 1
	}
	res, err := s.db.ExecContext(ctx, `UPDATE projects SET archived=?, rev=rev+1, updated=? WHERE id=?`, flag, s.stamp(), id)
	if err != nil {
		return Project{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Project{}, ErrNotFound
	}
	return s.GetProject(ctx, id)
}

// DeleteProject deletes an archived project and everything that belongs to it. It refuses with ErrRunActive while a
// run of the project is not terminal. Files under data/chibi_sensei are the caller's (ListRunIDs before, RemoveRunFiles after).
func (s *Store) DeleteProject(ctx context.Context, id, confirmName string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	p, err := loadProject(ctx, tx, id)
	if err != nil {
		return err
	}
	if !p.Archived {
		return ErrNotArchived
	}
	if confirmName != p.Name {
		return &ValidationError{Issues: []Issue{issue("confirm_name", "project.confirm_name", "The confirmation name does not match.")}}
	}
	var activeRuns int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM runs WHERE project_id=? AND state NOT IN `+terminalStatesSQL,
		id).Scan(&activeRuns); err != nil {
		return err
	}
	if activeRuns > 0 {
		return ErrRunActive
	}
	stmts := []string{
		`DELETE FROM run_events WHERE run_id IN (SELECT id FROM runs WHERE project_id=?)`,
		`DELETE FROM metrics WHERE run_id IN (SELECT id FROM runs WHERE project_id=?)`,
		`DELETE FROM eval_reports WHERE run_id IN (SELECT id FROM runs WHERE project_id=?)`,
		`DELETE FROM artifacts WHERE project_id=? OR run_id IN (SELECT id FROM runs WHERE project_id=?)`,
		`DELETE FROM runs WHERE project_id=?`,
		`DELETE FROM suggestions WHERE job_id IN (SELECT id FROM jobs WHERE project_id=?)`,
		`DELETE FROM dataset_rows WHERE project_id=?`,
		`DELETE FROM row_history WHERE project_id=?`,
		`DELETE FROM batches WHERE project_id=?`,
		`DELETE FROM documents WHERE project_id=?`,
		`DELETE FROM checks WHERE project_id=?`,
		`DELETE FROM jobs WHERE project_id=?`,
		`DELETE FROM projects WHERE id=?`,
	}
	for _, q := range stmts {
		args := []any{id}
		if strings.Count(q, "?") == 2 {
			args = append(args, id)
		}
		if _, err := tx.ExecContext(ctx, q, args...); err != nil {
			return err
		}
	}
	return tx.Commit()
}
