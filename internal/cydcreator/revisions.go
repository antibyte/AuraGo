package cydcreator

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"aurago/internal/fileutil"
)

// Revision reasons (overview "Revision" JSON).
const (
	RevisionBeforeJob       = "before_job"
	RevisionJob             = "job"
	RevisionRestore         = "restore"
	RevisionImport          = "import"
	RevisionTemplateUpgrade = "template_upgrade"

	// maxRevisionsPerProject revisions are kept per project; older rows are
	// deleted when a new one is created and their blobs pruned at startup.
	maxRevisionsPerProject = 50
	maxRevisionLabelRunes  = 80

	// maxBlobBytes bounds every blob read: the largest snapshot entries are
	// asset originals and the layout (read with one byte of slack, like
	// readLayout).
	maxBlobBytes = max(MaxLayoutBytes+1, MaxAssetUploadBytes, MaxAppFileBytes)
	// maxRevisionDiffBytes bounds the unified diffs of one RevisionDiff answer
	// (each file's diff is capped at maxDiffOutput on its own).
	maxRevisionDiffBytes = 1 << 20
)

// Revision is one saved project state (layout, src/app files and assets).
type Revision struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Reason    string `json:"reason"`
	JobID     string `json:"job_id"`
	BuildID   string `json:"build_id"`
	CreatedAt string `json:"created_at"`
}

// RevisionFileDiff is one changed file of a revision.
type RevisionFileDiff struct {
	Path   string `json:"path"`
	Status string `json:"status"` // added | modified | deleted
	Diff   string `json:"diff"`
}

// RevisionDiffResult is the GET /revisions/{id}/diff answer: what changed in
// this revision compared with the project's previous revision.
type RevisionDiffResult struct {
	LayoutChanged bool               `json:"layout_changed"`
	Files         []RevisionFileDiff `json:"files"`
}

// treeSnapshot is the revisioned content of a project tree.
type treeSnapshot struct {
	Layout []byte
	Files  map[string][]byte // src/app/** editable files and assets/<file>
}

// snapshotTree reads cyd.project.json, the editable app files and the asset
// originals of a project tree. A missing, oversized or non-regular layout is
// CYD_INVALID_LAYOUT (like readLayout), an app file above MaxAppFileBytes
// CYD_FILE_LIMIT and an asset file above MaxAssetUploadBytes
// CYD_ASSET_TOO_LARGE (reason file_size). Only regular files matching the
// asset file rule are taken from assets/.
func snapshotTree(root *os.Root) (treeSnapshot, error) {
	layout, err := rootReadFile(root, ProjectFile, MaxLayoutBytes+1)
	if err != nil {
		return treeSnapshot{}, layoutFileError(ProjectFile, fmt.Errorf("read %s: %w", ProjectFile, err))
	}
	files, err := readAppFiles(root)
	if err != nil {
		return treeSnapshot{}, err
	}
	entries, err := fs.ReadDir(root.FS(), "assets")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return treeSnapshot{}, fmt.Errorf("list assets: %w", err)
	}
	for _, e := range entries {
		rel := "assets/" + e.Name()
		if !e.Type().IsRegular() || !assetFileRegex.MatchString(rel) {
			continue
		}
		data, err := rootReadFile(root, rel, MaxAssetUploadBytes)
		if errors.Is(err, errFileTooLarge) {
			return treeSnapshot{}, NewError(CodeAssetTooLarge, fmt.Sprintf("%s is larger than %d bytes", rel, MaxAssetUploadBytes),
				map[string]any{"path": rel, "reason": "file_size", "max": MaxAssetUploadBytes})
		}
		if err != nil {
			return treeSnapshot{}, fmt.Errorf("read %s: %w", rel, err)
		}
		files[rel] = data
	}
	return treeSnapshot{Layout: layout, Files: files}, nil
}

// fingerprint identifies the snapshot content (layout and every file).
func (t treeSnapshot) fingerprint() string {
	paths := make([]string, 0, len(t.Files))
	for p := range t.Files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var b strings.Builder
	b.WriteString(ProjectFile + "\x00" + sha256Hex(t.Layout) + "\n")
	for _, p := range paths {
		b.WriteString(p + "\x00" + sha256Hex(t.Files[p]) + "\n")
	}
	return sha256Hex([]byte(b.String()))
}

// treeFingerprint snapshots dir and returns its fingerprint.
func treeFingerprint(dir string) (string, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return "", err
	}
	defer root.Close()
	snap, err := snapshotTree(root)
	if err != nil {
		return "", err
	}
	return snap.fingerprint(), nil
}

// The blob store <data>/cyd_creator/blobs/<aa>/<sha256> is only touched
// through an os.Root of the blobs directory, and a blob name is always a
// SHA-256 computed here or checked against blobName before use.

func (s *Service) blobsDir() string { return filepath.Join(s.dataDir, "cyd_creator", "blobs") }

var blobName = regexp.MustCompile(`^[0-9a-f]{64}$`)

// blobRel is the slash path of a blob below blobsDir; sha must be a valid
// blob name.
func blobRel(sha string) string { return sha[:2] + "/" + sha }

// blobPath is the absolute path of a blob (tests); sha must be a valid blob
// name.
func (s *Service) blobPath(sha string) string {
	return filepath.Join(s.blobsDir(), filepath.FromSlash(blobRel(sha)))
}

// openBlobRoot creates the blobs directory when needed and opens it.
func (s *Service) openBlobRoot() (*os.Root, error) {
	if err := os.MkdirAll(s.blobsDir(), 0o750); err != nil {
		return nil, fmt.Errorf("create blob directory: %w", err)
	}
	root, err := os.OpenRoot(s.blobsDir())
	if err != nil {
		return nil, fmt.Errorf("open blob directory: %w", err)
	}
	return root, nil
}

// writeBlob stores data content-addressed (deduplicated) and returns its
// hash. A new blob is a synced temporary sibling renamed into place.
func (s *Service) writeBlob(data []byte) (string, error) {
	sha := sha256Hex(data)
	root, err := s.openBlobRoot()
	if err != nil {
		return "", err
	}
	defer root.Close()
	final := filepath.FromSlash(blobRel(sha))
	if info, err := root.Lstat(final); err == nil && info.Mode().IsRegular() && info.Size() == int64(len(data)) {
		return sha, nil
	}
	if err := root.MkdirAll(sha[:2], 0o750); err != nil {
		return "", fmt.Errorf("create blob directory: %w", err)
	}
	var suffix [6]byte
	_, _ = rand.Read(suffix[:])
	tmp := final + ".tmp-" + hex.EncodeToString(suffix[:])
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return "", fmt.Errorf("create blob: %w", err)
	}
	_, werr := f.Write(data)
	if werr == nil {
		werr = f.Sync()
	}
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = fileutil.RenameRootContext(context.Background(), root, tmp, final)
	}
	if werr != nil {
		_ = root.Remove(tmp)
		return "", fmt.Errorf("write blob: %w", werr)
	}
	return sha, nil
}

// readBlob returns a blob (at most maxBlobBytes) after checking its hash.
func (s *Service) readBlob(sha string) ([]byte, error) {
	if !blobName.MatchString(sha) {
		return nil, fmt.Errorf("invalid blob name %q", clipEcho(sha))
	}
	root, err := os.OpenRoot(s.blobsDir())
	if err != nil {
		return nil, err
	}
	defer root.Close()
	data, err := rootReadFile(root, blobRel(sha), maxBlobBytes)
	if err != nil {
		return nil, err
	}
	if sha256Hex(data) != sha {
		return nil, fmt.Errorf("blob %s is corrupt", sha)
	}
	return data, nil
}

// revisionLabel shortens free text (a prompt or an earlier label) to one line.
func revisionLabel(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(text) > maxRevisionLabelRunes {
		text = string([]rune(text)[:maxRevisionLabelRunes-1]) + "…"
	}
	return text
}

// createRevision snapshots a project tree into the blob store and the
// revision tables, then trims the project to maxRevisionsPerProject.
func (s *Service) createRevision(ctx context.Context, projectID string, root *os.Root, label, reason, jobID, buildID string) (Revision, error) {
	snap, err := snapshotTree(root)
	if err != nil {
		return Revision{}, err
	}
	layoutSHA, err := s.writeBlob(snap.Layout)
	if err != nil {
		return Revision{}, err
	}
	type entry struct {
		path, sha string
		size      int
	}
	entries := make([]entry, 0, len(snap.Files))
	for p, data := range snap.Files {
		sha, err := s.writeBlob(data)
		if err != nil {
			return Revision{}, err
		}
		entries = append(entries, entry{p, sha, len(data)})
	}
	rev := Revision{ID: s.ids.NewID("ccr"), Label: revisionLabel(label), Reason: reason, JobID: jobID, BuildID: buildID,
		CreatedAt: formatTime(s.now())}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Revision{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `INSERT INTO cc_revisions(id,project_id,label,reason,job_id,build_id,layout_sha256,created_at)
		VALUES(?,?,?,?,?,?,?,?)`, rev.ID, projectID, rev.Label, rev.Reason, rev.JobID, rev.BuildID, layoutSHA, rev.CreatedAt); err != nil {
		return Revision{}, fmt.Errorf("insert revision: %w", err)
	}
	for _, e := range entries {
		if _, err := tx.ExecContext(ctx, `INSERT INTO cc_revision_files(revision_id,path,sha256,size) VALUES(?,?,?,?)`,
			rev.ID, e.path, e.sha, e.size); err != nil {
			return Revision{}, fmt.Errorf("insert revision file: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM cc_revision_files WHERE revision_id IN (SELECT id FROM cc_revisions
		WHERE project_id=? ORDER BY created_at DESC, id DESC LIMIT -1 OFFSET ?)`, projectID, maxRevisionsPerProject); err != nil {
		return Revision{}, fmt.Errorf("trim revision files: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM cc_revisions WHERE id IN (SELECT id FROM cc_revisions
		WHERE project_id=? ORDER BY created_at DESC, id DESC LIMIT -1 OFFSET ?)`, projectID, maxRevisionsPerProject); err != nil {
		return Revision{}, fmt.Errorf("trim revisions: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Revision{}, err
	}
	return rev, nil
}

// revisionRow is a revision with its project and layout hash.
type revisionRow struct {
	Revision
	ProjectID string
	LayoutSHA string
}

func (s *Service) loadRevision(ctx context.Context, id string) (revisionRow, error) {
	if !validElementID(id) {
		return revisionRow{}, NewError(CodeNotFound, "revision not found", map[string]any{"revision_id": clipEcho(id)})
	}
	var r revisionRow
	err := s.db.QueryRowContext(ctx, `SELECT id,project_id,label,reason,job_id,build_id,layout_sha256,created_at FROM cc_revisions WHERE id=?`, id).
		Scan(&r.ID, &r.ProjectID, &r.Label, &r.Reason, &r.JobID, &r.BuildID, &r.LayoutSHA, &r.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return revisionRow{}, NewError(CodeNotFound, "revision not found", map[string]any{"revision_id": id})
	}
	if err != nil {
		return revisionRow{}, fmt.Errorf("read revision: %w", err)
	}
	return r, nil
}

// revisionFiles returns path → sha256 of a revision.
func (s *Service) revisionFiles(ctx context.Context, id string) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT path, sha256 FROM cc_revision_files WHERE revision_id=?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	files := map[string]string{}
	for rows.Next() {
		var p, sha string
		if err := rows.Scan(&p, &sha); err != nil {
			return nil, err
		}
		files[p] = sha
	}
	return files, rows.Err()
}

// ListRevisions returns the project's revisions, newest first.
func (s *Service) ListRevisions(ctx context.Context, projectID string) ([]Revision, error) {
	if _, err := s.loadProjectRow(ctx, projectID); err != nil {
		return nil, s.internalError("list revisions", err)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,label,reason,job_id,build_id,created_at FROM cc_revisions
		WHERE project_id=? ORDER BY created_at DESC, id DESC`, projectID)
	if err != nil {
		return nil, s.internalError("list revisions", err)
	}
	defer rows.Close()
	out := []Revision{}
	for rows.Next() {
		var r Revision
		if err := rows.Scan(&r.ID, &r.Label, &r.Reason, &r.JobID, &r.BuildID, &r.CreatedAt); err != nil {
			return nil, s.internalError("list revisions", err)
		}
		out = append(out, r)
	}
	return out, s.internalError("list revisions", rows.Err())
}

// previousRevision returns the revision created before r in its project, or
// ok=false for the first one.
func (s *Service) previousRevision(ctx context.Context, r revisionRow) (revisionRow, bool, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM cc_revisions WHERE project_id=? AND (created_at<? OR (created_at=? AND id<?))
		ORDER BY created_at DESC, id DESC LIMIT 1`, r.ProjectID, r.CreatedAt, r.CreatedAt, r.ID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return revisionRow{}, false, nil
	}
	if err != nil {
		return revisionRow{}, false, err
	}
	prev, err := s.loadRevision(ctx, id)
	return prev, err == nil, err
}

// diffableText reports whether data can be shown as a line diff (UTF-8 text
// without NUL, the rule WriteFile applies to app files).
func diffableText(data []byte) bool {
	return utf8.Valid(data) && bytes.IndexByte(data, 0) < 0
}

// RevisionDiff compares a revision with the project's previous revision (the
// first revision is compared with an empty project). App files get unified
// diffs (each capped at maxDiffOutput, all together at about
// maxRevisionDiffBytes); assets and app files that are not UTF-8 text get a
// size note.
func (s *Service) RevisionDiff(ctx context.Context, revisionID string) (RevisionDiffResult, error) {
	r, err := s.loadRevision(ctx, revisionID)
	if err != nil {
		return RevisionDiffResult{}, s.internalError("diff revision", err)
	}
	files, err := s.revisionFiles(ctx, r.ID)
	if err != nil {
		return RevisionDiffResult{}, s.internalError("diff revision", err)
	}
	prev, hasPrev, err := s.previousRevision(ctx, r)
	if err != nil {
		return RevisionDiffResult{}, s.internalError("diff revision", err)
	}
	prevFiles := map[string]string{}
	if hasPrev {
		if prevFiles, err = s.revisionFiles(ctx, prev.ID); err != nil {
			return RevisionDiffResult{}, s.internalError("diff revision", err)
		}
	}
	out := RevisionDiffResult{LayoutChanged: !hasPrev || prev.LayoutSHA != r.LayoutSHA, Files: []RevisionFileDiff{}}
	paths := map[string]bool{}
	for p := range files {
		paths[p] = true
	}
	for p := range prevFiles {
		paths[p] = true
	}
	sorted := make([]string, 0, len(paths))
	for p := range paths {
		sorted = append(sorted, p)
	}
	sort.Strings(sorted)
	shown := 0
	for _, p := range sorted {
		newSHA, inNew := files[p]
		oldSHA, inOld := prevFiles[p]
		if inNew && inOld && newSHA == oldSHA {
			continue
		}
		status := "modified"
		switch {
		case !inOld:
			status = "added"
		case !inNew:
			status = "deleted"
		}
		var oldData, newData []byte
		if inOld {
			if oldData, err = s.readBlob(oldSHA); err != nil {
				return RevisionDiffResult{}, s.internalError("diff revision", err)
			}
		}
		if inNew {
			if newData, err = s.readBlob(newSHA); err != nil {
				return RevisionDiffResult{}, s.internalError("diff revision", err)
			}
		}
		diff := fmt.Sprintf("Binary file, %d → %d bytes\n", len(oldData), len(newData))
		if IsEditablePath(p) && diffableText(oldData) && diffableText(newData) {
			if shown >= maxRevisionDiffBytes {
				diff = "… diff omitted: this revision changes too much to show …\n"
			} else {
				diff = unifiedDiff(p, string(oldData), string(newData), status == "added", status == "deleted")
				shown += len(diff)
			}
		}
		out.Files = append(out.Files, RevisionFileDiff{Path: p, Status: status, Diff: diff})
	}
	return out, nil
}

// materializeRevision writes a revision's layout and files into the new,
// empty directory dir (through an os.Root of it). Only editable app paths and
// asset file paths are written; paths that differ only by letter case are
// refused (CYD_FILE_CONFLICT), because case-insensitive file systems would
// merge them.
func (s *Service) materializeRevision(ctx context.Context, r revisionRow, dir string) error {
	files, err := s.revisionFiles(ctx, r.ID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o750); err != nil {
		return err
	}
	if err := os.Mkdir(dir, 0o750); err != nil {
		return err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	missing := func(p string, cause error) error {
		s.logger.Warn("CYD Creator revision content could not be read", "revision_id", r.ID, "path", p, "error", cause)
		details := map[string]any{"revision_id": r.ID}
		if p != ProjectFile {
			details["path"] = p
		}
		return NewError(CodeNotFound, "the revision content is missing", details)
	}
	layout, err := s.readBlob(r.LayoutSHA)
	if err != nil {
		return missing(ProjectFile, err)
	}
	if err := rootWriteFile(context.WithoutCancel(ctx), root, ProjectFile, layout); err != nil {
		return err
	}
	folded := make(map[string]string, len(files))
	for p := range files {
		if !IsEditablePath(p) && !assetFileRegex.MatchString(p) {
			continue
		}
		key := strings.ToLower(p)
		if other, ok := folded[key]; ok {
			first, second := min(p, other), max(p, other)
			return NewError(CodeFileConflict, "the revision lists files whose names differ only in letter case",
				map[string]any{"revision_id": r.ID, "path": second, "existing": first})
		}
		folded[key] = p
	}
	for _, p := range folded {
		data, err := s.readBlob(files[p])
		if err != nil {
			return missing(p, err)
		}
		if err := rootWriteFile(context.WithoutCancel(ctx), root, p, data); err != nil {
			return err
		}
	}
	return nil
}

// layoutRowMeta returns the name, board and theme a published layout mirrors
// into its cc_projects row; like saveLayoutLocked, a value Validate rejects
// keeps the row's previous one.
func layoutRowMeta(row projectRow, p Project) (name, board, theme string) {
	name, board, theme = row.Name, row.Board, row.Theme
	if projectNameProblem(p.Name) == "" {
		name = p.Name
	}
	if _, err := BoardByID(p.Board); err == nil {
		board = p.Board
	}
	if _, err := ThemeByID(p.Theme); err == nil {
		theme = p.Theme
	}
	return name, board, theme
}

// RestoreRevision replaces the project's layout, app files and assets with a
// revision (crash-safe directory swap), records a "restore" revision and
// returns the new version. It respects the project lock (origin restore).
// The restored layout must parse and pass the asset entry rule
// (CYD_INVALID_LAYOUT). The project_changed events follow the commit, also
// when the restore revision cannot be recorded afterwards.
func (s *Service) RestoreRevision(ctx context.Context, revisionID string) (int64, Revision, error) {
	r, err := s.loadRevision(ctx, revisionID)
	if err != nil {
		return 0, Revision{}, s.internalError("restore revision", err)
	}
	unlock := s.lockProject(r.ProjectID)
	defer unlock()
	row, err := s.loadProjectRow(ctx, r.ProjectID)
	if err != nil {
		return 0, Revision{}, s.internalError("restore revision", err)
	}
	if err := checkLock(row, OriginRestore); err != nil {
		return 0, Revision{}, err
	}
	var suffix [4]byte
	_, _ = rand.Read(suffix[:])
	stageName := "restore-" + r.ID + "-" + hex.EncodeToString(suffix[:])
	stage := filepath.Join(s.stagingRoot(), stageName)
	if err := s.materializeRevision(ctx, r, stage); err != nil {
		_ = os.RemoveAll(stage)
		return 0, Revision{}, s.internalError("restore revision", err)
	}
	stageRoot, err := os.OpenRoot(stage)
	if err != nil {
		_ = os.RemoveAll(stage)
		return 0, Revision{}, s.internalError("restore revision", err)
	}
	p, err := readLayout(stageRoot)
	_ = stageRoot.Close()
	if err == nil {
		// The revision's asset files come with it, so only the entry rule
		// applies (file pattern, ids and files listed once).
		err = checkLayoutAssets(nil, p.Assets)
	}
	if err != nil {
		_ = os.RemoveAll(stage)
		return 0, Revision{}, s.internalError("restore revision", err)
	}
	name, board, theme := layoutRowMeta(row, p)
	version, err := s.publishStage(ctx, row, stage, receiptRestore, receiptRestore+"_"+strings.TrimPrefix(stageName, "restore-"),
		func(tx *sql.Tx, version int64) error {
			_, err := tx.ExecContext(ctx, `UPDATE cc_projects SET name=?, board=?, theme=?, app_changed_since_job=1 WHERE id=?`,
				name, board, theme, row.ID)
			return err
		})
	if err != nil {
		_ = os.RemoveAll(stage)
		return 0, Revision{}, s.internalError("restore revision", err)
	}
	// Committed: finish without the caller's cancellation.
	ctx = context.WithoutCancel(ctx)
	var rev Revision
	root, err := s.openRoot(row, "")
	if err == nil {
		rev, err = s.createRevision(ctx, row.ID, root, r.Label, RevisionRestore, "", "")
		_ = root.Close()
	}
	s.Emit(row.ID, EventProjectChanged, "", "", projectChange(version, OriginRestore, "layout"))
	s.Emit(row.ID, EventProjectChanged, "", "", projectChange(version, OriginRestore, "files"))
	if err != nil {
		return version, Revision{}, s.internalError("restore revision", err)
	}
	return version, rev, nil
}

// RecordRevision snapshots the current project directory as a revision with
// the given label and reason (one of the Revision* reasons). Plan 5's ZIP
// import records its "import" revision through it. It takes the project
// mutex, so callers must not hold it.
func (s *Service) RecordRevision(ctx context.Context, projectID, label, reason string) (Revision, error) {
	switch reason {
	case RevisionBeforeJob, RevisionJob, RevisionRestore, RevisionImport, RevisionTemplateUpgrade:
	default:
		return Revision{}, NewError(CodeBadRequest, fmt.Sprintf("unknown revision reason %q", clipEcho(reason)), nil)
	}
	unlock := s.lockProject(projectID)
	defer unlock()
	row, err := s.loadProjectRow(ctx, projectID)
	if err != nil {
		return Revision{}, s.internalError("record revision", err)
	}
	root, err := s.openRoot(row, "")
	if err != nil {
		return Revision{}, s.internalError("record revision", err)
	}
	defer root.Close()
	rev, err := s.createRevision(ctx, row.ID, root, label, reason, "", "")
	if err != nil {
		return Revision{}, s.internalError("record revision", err)
	}
	return rev, nil
}

// pruneOrphanBlobs removes blobs no revision references (deleted projects,
// trimmed revisions) and temporary blob writes. It runs once at startup,
// before anything can create a revision; a failed reference query aborts it,
// so a referenced blob is never removed.
func (s *Service) pruneOrphanBlobs(ctx context.Context) error {
	used := map[string]bool{}
	rows, err := s.db.QueryContext(ctx, `SELECT layout_sha256 FROM cc_revisions UNION SELECT sha256 FROM cc_revision_files`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var sha string
		if err := rows.Scan(&sha); err != nil {
			_ = rows.Close()
			return err
		}
		used[sha] = true
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	root, err := os.OpenRoot(s.blobsDir())
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer root.Close()
	return fs.WalkDir(root.FS(), ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := path.Base(rel)
		if strings.Contains(name, ".tmp-") || (blobName.MatchString(name) && !used[name]) {
			_ = root.Remove(filepath.FromSlash(rel))
		}
		return nil
	})
}
