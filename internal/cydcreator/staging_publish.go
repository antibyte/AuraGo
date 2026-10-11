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
	"strings"

	"aurago/internal/fileutil"
)

// A synced receipt precedes every swap of a project directory (pattern
// internal/gamemaker/fs_receipt.go). The database version is the commit
// point: version == NewVersion means the swap is committed, version ==
// PreviousVersion means it is not; anything else is left for review. A
// receipt that outlives its commit (the backup could not be removed) is
// rewritten with Committed set, so later version bumps cannot make it
// ambiguous.
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
	Committed       bool   `json:"committed,omitempty"`
}

var (
	receiptIDPattern = regexp.MustCompile(`^(publish|restore)_[A-Za-z0-9_-]{1,96}$`)
	slugPattern      = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,49}$`)
	stagePattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)
)

func (s *Service) receiptsDir() string {
	return filepath.Join(s.dataDir, "cyd_creator", "transactions")
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
	return p, nil
}

// markReceiptCommitted replaces the receipt at p with one that records the
// commit (best effort: an unmarked receipt is still recovered while the
// version stays NewVersion).
func (s *Service) markReceiptCommitted(p string, r publishReceipt) {
	r.Committed = true
	data, err := json.Marshal(r)
	if err == nil {
		err = fileutil.WriteFileContext(context.Background(), p, data, 0o600)
	}
	if err != nil {
		s.logger.Warn("CYD Creator receipt could not be marked as committed", "receipt", r.ID, "error", err)
	}
}

// publishStage replaces the project directory with stage. commit runs inside
// the transaction that bumps the version (the commit point); on any failure
// before the commit the old directory is put back. Callers hold the project
// mutex. It returns the new version.
func (s *Service) publishStage(ctx context.Context, row projectRow, stage, kind, id string, commit func(tx *sql.Tx, version int64) error) (int64, error) {
	r := publishReceipt{Version: receiptFormat, ID: id, Kind: kind, ProjectID: row.ID, Slug: row.Slug,
		Stage: filepath.Base(stage), PreviousVersion: row.Version, NewVersion: row.Version + 1}
	target, backup, stagePath, err := s.receiptPaths(r)
	if err != nil {
		return 0, err
	}
	if filepath.Clean(stagePath) != filepath.Clean(stage) {
		return 0, errors.New("the stage is outside the staging folder")
	}
	receipt, err := s.writeReceipt(r)
	if err != nil {
		return 0, err
	}
	if err := fileutil.Rename(target, backup); err != nil {
		_ = os.Remove(receipt)
		return 0, fmt.Errorf("move project aside: %w", err)
	}
	rollback := func(cause error) (int64, error) {
		if dirExists(target) {
			_ = fileutil.Rename(target, stage)
		}
		if err := fileutil.Rename(backup, target); err != nil {
			s.logger.Error("CYD Creator project could not be put back; the receipt stays for recovery", "receipt", r.ID, "error", err)
			return 0, cause
		}
		_ = os.Remove(receipt)
		return 0, cause
	}
	if err := fileutil.Rename(stage, target); err != nil {
		return rollback(fmt.Errorf("move stage into place: %w", err))
	}
	tx, err := s.db.BeginTx(context.WithoutCancel(ctx), nil)
	if err != nil {
		return rollback(err)
	}
	res, err := tx.Exec(`UPDATE cc_projects SET version=?, updated_at=? WHERE id=? AND version=?`,
		r.NewVersion, formatTime(s.now()), row.ID, row.Version)
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
	if err := os.RemoveAll(backup); err != nil {
		s.logger.Warn("CYD Creator publication backup could not be removed", "receipt", r.ID, "error", err)
		s.markReceiptCommitted(receipt, r)
		return r.NewVersion, nil // the receipt stays; recovery removes the backup
	}
	_ = os.Remove(receipt)
	return r.NewVersion, nil
}

// recoverReceipts finishes or rolls back swaps interrupted by a crash.
// Ambiguous receipts are kept and logged; the project stays as found.
func (s *Service) recoverReceipts(ctx context.Context) {
	entries, err := os.ReadDir(s.receiptsDir())
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			s.logger.Error("CYD Creator receipts could not be listed", "error", err)
		}
		return
	}
	if len(entries) > maxReceipts {
		s.logger.Error("CYD Creator has too many receipts; recovery skipped", "count", len(entries))
		return
	}
	for _, e := range entries {
		p := filepath.Join(s.receiptsDir(), e.Name())
		if strings.Contains(e.Name(), ".json.tmp") || strings.HasPrefix(e.Name(), ".") {
			_ = os.Remove(p) // an unfinished receipt write: no swap started
			continue
		}
		if err := s.recoverReceipt(ctx, p); err != nil {
			s.logger.Error("CYD Creator receipt needs review", "receipt", e.Name(), "error", err)
		}
	}
}

func (s *Service) recoverReceipt(ctx context.Context, p string) error {
	info, err := os.Lstat(p)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 16<<10 || !strings.HasSuffix(p, ".json") {
		return errors.New("unrecognized receipt file")
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	var r publishReceipt
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	if filepath.Base(p) != r.ID+".json" {
		return errors.New("receipt name mismatch")
	}
	target, backup, stage, err := s.receiptPaths(r)
	if err != nil {
		return err
	}
	var version int64
	var slug string
	if err := s.db.QueryRowContext(ctx, `SELECT version, slug FROM cc_projects WHERE id=?`, r.ProjectID).Scan(&version, &slug); err != nil {
		return fmt.Errorf("read project: %w", err)
	}
	if slug != r.Slug {
		return errors.New("the project directory changed")
	}
	switch {
	case version == r.NewVersion, r.Committed && version > r.NewVersion:
		if !dirExists(target) {
			return errors.New("the committed project directory is missing")
		}
		if err := os.RemoveAll(backup); err != nil {
			return err
		}
	case !r.Committed && version == r.PreviousVersion:
		if dirExists(backup) {
			if dirExists(target) {
				if dirExists(stage) {
					return errors.New("both the stage and the new project directory exist")
				}
				if err := fileutil.Rename(target, stage); err != nil {
					return err
				}
			}
			if err := fileutil.Rename(backup, target); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("project version %d matches neither side of the receipt", version)
	}
	return os.Remove(p)
}
