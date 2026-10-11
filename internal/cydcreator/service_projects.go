package cydcreator

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// ProjectFile is the layout file in every project directory.
const ProjectFile = "cyd.project.json"

const (
	defaultProjectTheme = "clear"
	maxSlugBytes        = 40
	maxSlugAttempts     = 999
)

// ProjectSummary is the project metadata of the API.
type ProjectSummary struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Board       string `json:"board"`
	Theme       string `json:"theme"`
	Version     int64  `json:"version"`
	LockedByJob string `json:"locked_by_job"`
	HasDraft    bool   `json:"has_draft"`
	LastBuild   *Build `json:"last_build"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// CreateProjectInput is the body of POST /projects. Theme defaults to
// "clear", Language to "en" and Timezone to "UTC0".
type CreateProjectInput struct {
	Name     string `json:"name"`
	Board    string `json:"board"`
	Theme    string `json:"theme"`
	Language string `json:"language"`
	Timezone string `json:"timezone"`
}

// asciiFold lowercases s and maps letters to ASCII (ä→ae, é→e).
func asciiFold(s string) string {
	replacer := strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss", "Ä", "ae", "Ö", "oe", "Ü", "ue",
		"æ", "ae", "Æ", "ae", "ø", "o", "Ø", "o", "å", "a", "Å", "a", "ł", "l", "Ł", "l", "đ", "d", "Đ", "d")
	decomposed := norm.NFD.String(replacer.Replace(s))
	var b strings.Builder
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// slugify turns a project name into a directory name of [a-z0-9-], at most
// maxSlugBytes long; names without usable characters become "project". A
// Windows device name (nul, con, com1, ...) gets "-project" appended: on
// Windows os.Mkdir("nul") succeeds without creating anything.
func slugify(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range asciiFold(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > maxSlugBytes {
		slug = strings.TrimRight(slug[:maxSlugBytes], "-")
	}
	if slug == "" {
		slug = "project"
	}
	if windowsReserved(slug) {
		slug += "-project"
	}
	return slug
}

func validProjectName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > maxProjectNameRunes ||
		strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return "", NewError(CodeBadRequest,
			fmt.Sprintf("project name must be 1..%d characters without control characters", maxProjectNameRunes), nil)
	}
	return name, nil
}

// allocateProjectDir creates <projects>/<slug>[-n]/ exclusively and returns
// the slug. Callers hold createMu.
func (s *Service) allocateProjectDir(ctx context.Context, name string) (string, error) {
	base := slugify(name)
	// <workspace>/CYD may have been deleted since NewService created it.
	if err := os.MkdirAll(s.projectsDir, 0o750); err != nil {
		return "", fmt.Errorf("create projects directory: %w", err)
	}
	for n := 1; n <= maxSlugAttempts; n++ {
		slug := base
		if n > 1 {
			suffix := fmt.Sprintf("-%d", n)
			slug = strings.TrimRight(base[:min(len(base), maxSlugBytes-len(suffix))], "-") + suffix
		}
		var exists int
		err := s.db.QueryRowContext(ctx, `SELECT 1 FROM cc_projects WHERE slug=?`, slug).Scan(&exists)
		if err == nil {
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("check project slug: %w", err)
		}
		if err := os.Mkdir(s.projectDir(slug), 0o750); err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return "", fmt.Errorf("create project directory: %w", err)
		}
		return slug, nil
	}
	return "", NewError(CodeBusy, "no free project directory name", map[string]any{"slug": base})
}

// CreateProject creates the project directory <workspace>/CYD/<slug>/ with
// cyd.project.json and the scaffolded src/app files, and the database row.
func (s *Service) CreateProject(ctx context.Context, in CreateProjectInput) (ProjectSummary, error) {
	theme := in.Theme
	if theme == "" {
		theme = defaultProjectTheme
	}
	p, err := NewProject(in.Name, in.Board, theme, in.Language, in.Timezone, s.ids)
	if err != nil {
		return ProjectSummary{}, err
	}
	layout, err := MarshalProject(p)
	if err != nil {
		return ProjectSummary{}, s.internalError("create project", err)
	}
	s.createMu.Lock()
	defer s.createMu.Unlock()
	slug, err := s.allocateProjectDir(ctx, p.Name)
	if err != nil {
		return ProjectSummary{}, s.internalError("create project", err)
	}
	dir := s.projectDir(slug)
	fail := func(err error) (ProjectSummary, error) {
		_ = os.RemoveAll(dir)
		return ProjectSummary{}, s.internalError("create project", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return fail(err)
	}
	defer root.Close()
	if err := rootWriteFile(ctx, root, ProjectFile, layout); err != nil {
		return fail(err)
	}
	for rel, data := range ScaffoldAppFiles(p) {
		if !IsEditablePath(rel) {
			continue
		}
		if err := rootWriteFile(ctx, root, rel, data); err != nil {
			return fail(err)
		}
	}
	now := formatTime(s.now())
	row := projectRow{ID: s.ids.NewID("ccp"), Slug: slug, Name: p.Name, Board: p.Board, Theme: p.Theme, Version: 1,
		CreatedAt: now, UpdatedAt: now}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO cc_projects(id,slug,name,board,theme,version,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?)`, row.ID, row.Slug, row.Name, row.Board, row.Theme, row.Version, row.CreatedAt, row.UpdatedAt); err != nil {
		return fail(err)
	}
	return s.summary(ctx, row), nil
}

// summary converts a row; last_build is resolved when set.
func (s *Service) summary(ctx context.Context, row projectRow) ProjectSummary {
	out := ProjectSummary{ID: row.ID, Slug: row.Slug, Name: row.Name, Board: row.Board, Theme: row.Theme,
		Version: row.Version, LockedByJob: row.LockedByJob, HasDraft: row.DraftJob != "",
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
	if row.LastBuild != "" {
		if b, err := s.loadBuild(ctx, row.LastBuild); err == nil {
			out.LastBuild = &b
		}
	}
	return out
}

// ListProjects returns every project, most recently changed first.
func (s *Service) ListProjects(ctx context.Context) ([]ProjectSummary, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+projectColumns+` FROM cc_projects ORDER BY updated_at DESC, id`)
	if err != nil {
		return nil, s.internalError("list projects", err)
	}
	var list []projectRow
	for rows.Next() {
		row, err := scanProjectRow(rows)
		if err != nil {
			_ = rows.Close()
			return nil, s.internalError("list projects", err)
		}
		list = append(list, row)
	}
	if err := rows.Close(); err != nil {
		return nil, s.internalError("list projects", err)
	}
	out := make([]ProjectSummary, 0, len(list))
	for _, row := range list {
		out = append(out, s.summary(ctx, row))
	}
	return out, nil
}

// GetProject returns one project or CYD_NOT_FOUND.
func (s *Service) GetProject(ctx context.Context, id string) (ProjectSummary, error) {
	row, err := s.loadProjectRow(ctx, id)
	if err != nil {
		return ProjectSummary{}, s.internalError("read project", err)
	}
	return s.summary(ctx, row), nil
}

// RenameProject changes the display name in the database and in
// cyd.project.json (the slug and directory stay) and bumps the version.
func (s *Service) RenameProject(ctx context.Context, id, name string) (ProjectSummary, error) {
	name, err := validProjectName(name)
	if err != nil {
		return ProjectSummary{}, err
	}
	unlock := s.lockProject(id)
	defer unlock()
	row, err := s.loadProjectRow(ctx, id)
	if err != nil {
		return ProjectSummary{}, s.internalError("rename project", err)
	}
	if err := checkLock(row, OriginUser); err != nil {
		return ProjectSummary{}, err
	}
	root, err := s.openProjectRoot(row)
	if err != nil {
		return ProjectSummary{}, s.internalError("rename project", err)
	}
	defer root.Close()
	p, err := readLayout(root)
	if err != nil {
		return ProjectSummary{}, s.internalError("rename project", err)
	}
	p.Name = name
	version, err := s.saveLayoutLocked(ctx, root, row, p)
	if err != nil {
		return ProjectSummary{}, s.internalError("rename project", err)
	}
	s.Emit(id, EventProjectChanged, "", "", projectChange(version, OriginUser, "meta"))
	return s.GetProject(ctx, id)
}

// DeleteProject removes the project directory, its artefacts and every row
// that belongs to it. A queued or running build refuses with CYD_BUSY.
func (s *Service) DeleteProject(ctx context.Context, id string) error {
	unlock := s.lockProject(id)
	defer unlock()
	row, err := s.loadProjectRow(ctx, id)
	if err != nil {
		return s.internalError("delete project", err)
	}
	if err := checkLock(row, OriginUser); err != nil {
		return err
	}
	active, err := s.projectHasActiveBuild(ctx, id)
	if err != nil {
		return s.internalError("delete project", err)
	}
	if active {
		return NewError(CodeBusy, "a build of this project is queued or running", map[string]any{"project_id": id})
	}
	buildIDs, err := s.projectBuildIDs(ctx, id)
	if err != nil {
		return s.internalError("delete project", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return s.internalError("delete project", err)
	}
	statements := []string{
		`DELETE FROM cc_revision_files WHERE revision_id IN (SELECT id FROM cc_revisions WHERE project_id=?)`,
		`DELETE FROM cc_revisions WHERE project_id=?`,
		`DELETE FROM cc_builds WHERE project_id=?`,
		`DELETE FROM cc_events WHERE project_id=?`,
		`DELETE FROM cc_jobs WHERE project_id=?`,
		`DELETE FROM cc_messages WHERE project_id=?`,
		`DELETE FROM cc_agent_context WHERE project_id=?`,
		`DELETE FROM cc_devices WHERE project_id=?`,
		`DELETE FROM cc_projects WHERE id=?`,
	}
	for _, stmt := range statements {
		if _, err := tx.ExecContext(ctx, stmt, id); err != nil {
			_ = tx.Rollback()
			return s.internalError("delete project", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return s.internalError("delete project", err)
	}
	for _, buildID := range buildIDs {
		_ = os.RemoveAll(filepath.Join(s.artifactsDir, buildID))
	}
	if err := os.RemoveAll(s.projectDir(row.Slug)); err != nil {
		s.logger.Warn("CYD Creator project directory could not be removed", "project_id", id, "error", err)
	}
	s.dropProjectSubscribers(id)
	return nil
}

func (s *Service) projectBuildIDs(ctx context.Context, projectID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM cc_builds WHERE project_id=?`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// projectChange is the data of a project_changed event.
func projectChange(version int64, origin, what string) map[string]any {
	return map[string]any{"version": version, "origin": origin, "what": what}
}

// projectHasActiveBuild reports a queued or running build of the project.
// StartBuild inserts its row under the same project lock, so the database is
// authoritative here. A failed query is an error, never "no build".
func (s *Service) projectHasActiveBuild(ctx context.Context, projectID string) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM cc_builds WHERE project_id=? AND status IN ('queued','running') LIMIT 1`, projectID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check active builds: %w", err)
	}
	return true, nil
}
