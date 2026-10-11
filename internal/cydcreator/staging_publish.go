package cydcreator

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"

	"aurago/internal/fileutil"
)

// A synced receipt precedes every swap of a project directory (pattern
// internal/gamemaker/fs_receipt.go). The commit point is the transaction that
// bumps the version and stores the receipt id in cc_projects.publish_receipt:
// recovery treats a receipt as committed exactly when the row names it. A
// receipt that is not committed is rolled back only while the version is
// still PreviousVersion; anything else is left for review. While a receipt of
// a project is unresolved (a failed put-back, a recovery error) the project is
// fenced in memory: every write refuses with CYD_BUSY (reason
// publication_review) until a restart resolves it.
const (
	receiptPublish = "publish"
	receiptRestore = "restore"
	receiptFormat  = 1
	maxReceipts    = 1000
)

type publishReceipt struct {
	Version         int    `json:"version"`
	ID              string `json:"id"`
	Kind            string `json:"kind"`
	ProjectID       string `json:"project_id"`
	Slug            string `json:"slug"`
	Stage           string `json:"stage"`
	PreviousVersion int64  `json:"previous_version"`
	NewVersion      int64  `json:"new_version"`
}

var (
	receiptIDPattern = regexp.MustCompile(`^(publish|restore)_[A-Za-z0-9_-]{1,96}$`)
	slugPattern      = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,49}$`)
	stagePattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)
	backupPattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,49}\.cyd-backup-((?:publish|restore)_[A-Za-z0-9_-]{1,96})$`)
)

func (s *Service) receiptsDir() string {
	return filepath.Join(s.dataDir, "cyd_creator", "transactions")
}

// publicationFence returns the receipt id of an unresolved publication of
// the project, or "".
func (s *Service) publicationFence(projectID string) string {
	s.fenceMu.Lock()
	defer s.fenceMu.Unlock()
	return s.fences[projectID]
}

// setPublicationFence fences the project until the next start.
func (s *Service) setPublicationFence(projectID, receiptID string) {
	s.fenceMu.Lock()
	defer s.fenceMu.Unlock()
	if s.fences == nil {
		s.fences = map[string]string{}
	}
	s.fences[projectID] = receiptID
}

func publicationReviewError() error {
	return NewError(CodeBusy, "an interrupted publication of this project needs review; restart AuraGo or see the server log",
		map[string]any{"reason": "publication_review"})
}

// isOpenFileError reports a Windows directory rename refused because a file
// or folder inside is open (access denied or sharing violation).
func isOpenFileError(err error) bool {
	return runtime.GOOS == "windows" && (errors.Is(err, fs.ErrPermission) || errors.Is(err, syscall.Errno(32)))
}

// swapError maps a failed directory rename: an open file is CYD_BUSY (reason
// file_open, retryable), anything else stays internal.
func swapError(what string, err error) error {
	if isOpenFileError(err) {
		return NewError(CodeBusy, "a project file is open; try again in a moment", map[string]any{"reason": "file_open"})
	}
	return fmt.Errorf("%s: %w", what, err)
}

// syncDir makes new entries and renames in dir durable. Windows has no
// directory fsync (NTFS journals its metadata), so it is a no-op there, and a
// file system without directory fsync (EINVAL) is accepted.
func syncDir(dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	err = f.Sync()
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if errors.Is(err, syscall.EINVAL) || errors.Is(err, errors.ErrUnsupported) {
		return nil
	}
	return err
}

// syncTree fsyncs every directory of the tree at dir; its files were synced
// when they were written. A no-op on Windows.
func syncTree(dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	return filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		return syncDir(p)
	})
}

// rejectLinkedComponents fails when a component of the slash path rel below
// base exists as a symlink or another reparse point (a Windows junction is
// irregular); the check ends at the first missing component.
func rejectLinkedComponents(base, rel string) error {
	current := base
	for _, part := range strings.Split(rel, "/") {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
			return fmt.Errorf("%s is a link or reparse point", rel)
		}
	}
	return nil
}

// receiptPaths validates a receipt and returns the target, backup and stage
// directories it may touch. Every name comes from a validated receipt field,
// and no existing component of the three paths below the workspace may be a
// link.
func (s *Service) receiptPaths(r publishReceipt) (target, backup, stage string, err error) {
	if r.Version != receiptFormat || !receiptIDPattern.MatchString(r.ID) || !strings.HasPrefix(r.ID, r.Kind+"_") ||
		!slugPattern.MatchString(r.Slug) || !stagePattern.MatchString(r.Stage) || r.ProjectID == "" ||
		r.NewVersion != r.PreviousVersion+1 {
		return "", "", "", errors.New("invalid cyd creator receipt")
	}
	projects := filepath.Base(s.projectsDir)
	for _, rel := range []string{projects + "/" + r.Slug, projects + "/" + r.Slug + ".cyd-backup-" + r.ID, stagingDirName + "/" + r.Stage} {
		if err := rejectLinkedComponents(s.workspaceDir, rel); err != nil {
			return "", "", "", fmt.Errorf("cyd creator receipt path: %w", err)
		}
	}
	target = s.projectDir(r.Slug)
	return target, target + ".cyd-backup-" + r.ID, filepath.Join(s.stagingRoot(), r.Stage), nil
}

// writeReceipt persists the receipt durably (synced file, synced directory).
func (s *Service) writeReceipt(r publishReceipt) (string, error) {
	if _, _, _, err := s.receiptPaths(r); err != nil {
		return "", err
	}
	if err := os.MkdirAll(s.receiptsDir(), 0o700); err != nil {
		return "", err
	}
	p := filepath.Join(s.receiptsDir(), r.ID+".json")
	if _, err := os.Lstat(p); !errors.Is(err, os.ErrNotExist) {
		return "", errors.New("the receipt already exists")
	}
	data, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	if err := fileutil.WriteFileContext(context.Background(), p, data, 0o600); err != nil {
		return "", fmt.Errorf("persist receipt: %w", err)
	}
	if err := syncDir(s.receiptsDir()); err != nil {
		_ = os.Remove(p)
		return "", fmt.Errorf("persist receipt: %w", err)
	}
	return p, nil
}

// publishStage replaces the project directory with stage. commit runs inside
// the transaction that bumps the version and records the receipt id (the
// commit point); on any failure before the commit the old directory is put
// back. A rename refused because a file is open is CYD_BUSY (reason
// file_open); callers may retry. Callers hold the project mutex. It returns
// the new version.
func (s *Service) publishStage(ctx context.Context, row projectRow, stage, kind, id string, commit func(tx *sql.Tx, version int64) error) (int64, error) {
	if s.publicationFence(row.ID) != "" {
		return 0, publicationReviewError()
	}
	r := publishReceipt{Version: receiptFormat, ID: id, Kind: kind, ProjectID: row.ID, Slug: row.Slug,
		Stage: filepath.Base(stage), PreviousVersion: row.Version, NewVersion: row.Version + 1}
	target, backup, stagePath, err := s.receiptPaths(r)
	if err != nil {
		return 0, err
	}
	if filepath.Clean(stagePath) != filepath.Clean(stage) {
		return 0, errors.New("the stage is outside the staging folder")
	}
	// The stage's files were synced when written; its directories now, so
	// the published tree survives a power loss once the backup is gone.
	if err := syncTree(stage); err != nil {
		return 0, fmt.Errorf("sync stage: %w", err)
	}
	receipt, err := s.writeReceipt(r)
	if err != nil {
		return 0, err
	}
	if err := fileutil.Rename(target, backup); err != nil {
		_ = os.Remove(receipt)
		return 0, swapError("move project aside", err)
	}
	rollback := func(cause error) (int64, error) {
		if dirExists(target) {
			_ = fileutil.Rename(target, stage)
		}
		if err := fileutil.Rename(backup, target); err != nil {
			s.setPublicationFence(row.ID, r.ID)
			s.logger.Error("CYD Creator project could not be put back; the receipt stays for recovery at the next start",
				"receipt", r.ID, "project_id", row.ID, "error", err, "cause", cause)
			return 0, publicationReviewError()
		}
		_ = os.Remove(receipt)
		return 0, cause
	}
	if err := fileutil.Rename(stage, target); err != nil {
		return rollback(swapError("move stage into place", err))
	}
	if err := errors.Join(syncDir(s.projectsDir), syncDir(s.stagingRoot())); err != nil {
		return rollback(fmt.Errorf("sync the swapped directories: %w", err))
	}
	tx, err := s.db.BeginTx(context.WithoutCancel(ctx), nil)
	if err != nil {
		return rollback(err)
	}
	res, err := tx.Exec(`UPDATE cc_projects SET version=?, updated_at=?, publish_receipt=? WHERE id=? AND version=?`,
		r.NewVersion, formatTime(s.now()), r.ID, row.ID, row.Version)
	if err == nil {
		if n, _ := res.RowsAffected(); n != 1 {
			err = errors.New("the project version changed during publication")
		}
	}
	if err == nil && commit != nil {
		err = commit(tx, r.NewVersion)
	}
	if err == nil {
		err = tx.Commit()
	} else {
		_ = tx.Rollback()
	}
	if err != nil {
		return rollback(err)
	}
	// Committed. A backup that cannot be removed now has no receipt left and
	// is swept at the next start (sweepOrphanBackups).
	if err := os.RemoveAll(backup); err != nil {
		s.logger.Warn("CYD Creator publication backup could not be removed; it is removed at the next start", "receipt", r.ID, "error", err)
	}
	_ = os.Remove(receipt)
	return r.NewVersion, nil
}

// receiptRecovery is what recoverReceipts found.
type receiptRecovery struct {
	// Listed is false when the receipts folder could not be read or held more
	// than maxReceipts entries: then no receipt was resolved.
	Listed bool
	// Complete is false when a receipt left for review names no project or
	// no valid stage, so its project is unknown.
	Complete bool
	// Stages are the stage folders of the receipts left for review.
	Stages map[string]bool
	// Committed are receipts of committed swaps whose cleanup was cut short
	// by the crash; they are resolved now.
	Committed []publishReceipt
}

// recoverReceipts finishes or rolls back swaps interrupted by a crash, then
// removes backups whose receipt is gone. Receipts that need review are kept
// and logged, the project stays as found and is fenced until the next start.
// The folder is listed once; the result tells the caller what is left.
func (s *Service) recoverReceipts(ctx context.Context) receiptRecovery {
	out := receiptRecovery{Stages: map[string]bool{}}
	entries, err := os.ReadDir(s.receiptsDir())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		s.logger.Error("CYD Creator receipts could not be listed", "error", err)
		return out
	}
	if len(entries) > maxReceipts {
		s.logger.Error("CYD Creator has too many receipts; recovery skipped", "count", len(entries))
		return out
	}
	out.Listed, out.Complete = true, true
	for _, e := range entries {
		p := filepath.Join(s.receiptsDir(), e.Name())
		if strings.Contains(e.Name(), ".json.tmp") || strings.HasPrefix(e.Name(), ".") {
			_ = os.Remove(p) // an unfinished receipt write: no swap started
			continue
		}
		r, committed, err := s.recoverReceipt(ctx, p)
		if err != nil {
			if r.ProjectID != "" {
				s.setPublicationFence(r.ProjectID, r.ID)
			}
			if r.ProjectID == "" || !stagePattern.MatchString(r.Stage) {
				out.Complete = false
			} else {
				out.Stages[r.Stage] = true
			}
			s.logger.Error("CYD Creator receipt needs review", "receipt", e.Name(), "project_id", r.ProjectID, "error", err)
			continue
		}
		if committed {
			out.Committed = append(out.Committed, r)
		}
	}
	s.sweepOrphanBackups()
	return out
}

// recoverReceipt resolves one receipt file and reports whether it was a
// committed swap. It returns the parsed receipt (empty when the file is
// unreadable or misnamed) so the caller can fence its project.
func (s *Service) recoverReceipt(ctx context.Context, p string) (publishReceipt, bool, error) {
	info, err := os.Lstat(p)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 16<<10 || !strings.HasSuffix(p, ".json") {
		return publishReceipt{}, false, errors.New("unrecognized receipt file")
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return publishReceipt{}, false, err
	}
	var r publishReceipt
	if err := json.Unmarshal(data, &r); err != nil {
		return publishReceipt{}, false, err
	}
	if filepath.Base(p) != r.ID+".json" {
		return publishReceipt{}, false, errors.New("receipt name mismatch")
	}
	target, backup, stage, err := s.receiptPaths(r)
	if err != nil {
		return r, false, err
	}
	var version int64
	var slug, committed string
	if err := s.db.QueryRowContext(ctx, `SELECT version, slug, publish_receipt FROM cc_projects WHERE id=?`, r.ProjectID).
		Scan(&version, &slug, &committed); err != nil {
		return r, false, fmt.Errorf("read project: %w", err)
	}
	if slug != r.Slug {
		return r, false, errors.New("the project directory changed")
	}
	switch {
	case committed == r.ID:
		// Committed; saves after the commit do not matter.
		if !dirExists(target) {
			return r, false, errors.New("the committed project directory is missing")
		}
		if err := os.RemoveAll(backup); err != nil {
			return r, false, err
		}
		return r, true, os.Remove(p)
	case version == r.PreviousVersion:
		// Not committed, and nothing was saved since (a commit always bumps
		// the version): put the old directory back.
		if dirExists(backup) {
			if dirExists(target) {
				if dirExists(stage) {
					return r, false, errors.New("both the stage and the new project directory exist")
				}
				if err := fileutil.Rename(target, stage); err != nil {
					return r, false, err
				}
			}
			if err := fileutil.Rename(backup, target); err != nil {
				return r, false, err
			}
		}
	default:
		return r, false, fmt.Errorf("the receipt is not committed but the project version moved from %d to %d", r.PreviousVersion, version)
	}
	return r, false, os.Remove(p)
}

// sweepOrphanBackups removes <slug>.cyd-backup-<receipt> directories whose
// receipt no longer exists: their publication was committed (the receipt is
// removed only after a commit or a completed rollback).
func (s *Service) sweepOrphanBackups() {
	entries, err := os.ReadDir(s.projectsDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		m := backupPattern.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		if _, err := os.Lstat(filepath.Join(s.receiptsDir(), m[1]+".json")); !errors.Is(err, fs.ErrNotExist) {
			continue // the receipt is still pending (or cannot be checked)
		}
		if err := os.RemoveAll(filepath.Join(s.projectsDir, e.Name())); err != nil {
			s.logger.Warn("CYD Creator publication backup could not be removed", "backup", e.Name(), "error", err)
		}
	}
}
