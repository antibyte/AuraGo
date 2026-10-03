package gamemaker

import (
	"aurago/internal/fileutil"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type revisionFile struct {
	Path string
	Hash string
	Size int64
}

func (s *Service) ListRevisions(ctx context.Context, projectID string) ([]Revision, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,project_id,number,parent,source,summary,file_count,total_bytes,created_at
		FROM gm_revisions WHERE project_id=? ORDER BY number DESC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list game maker revisions: %w", err)
	}
	defer rows.Close()
	var out []Revision
	for rows.Next() {
		revision, err := scanRevision(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, revision)
	}
	return out, rows.Err()
}

// Keep the admission check and filesystem commit under the same file, policy,
// and build locks. Preserve that order to match policy-guarded asset imports,
// which acquire the build lock while holding the policy read lock.
// Cancellation, a new build or late diagnostics revoke publication.
func (s *Service) publishValidated(ctx context.Context, stage string, project Project, job Job, result BuildResult) (Revision, error) {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	s.policyMu.RLock()
	defer s.policyMu.RUnlock()
	s.buildMu.Lock()
	defer s.buildMu.Unlock()
	if !s.policy.Enabled {
		return Revision{}, ErrDisabled
	}
	if s.policy.ReadOnly || !s.policy.AllowEdit {
		return Revision{}, ErrReadOnly
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Revision{}, err
	}
	if !result.OK || result.check == nil || result.check != s.previewCheck || s.activeJobID != job.ID {
		return Revision{}, fmt.Errorf("publication requires the current validated build")
	}
	if len(result.check.Diagnostics) > 0 {
		return Revision{}, fmt.Errorf("preview failed before publication: %s", diagnosticsText(result.check.Diagnostics))
	}
	var plan GamePlan
	if project.Dimension == "3d" {
		data, err := os.ReadFile(filepath.Join(stage, filepath.FromSlash(gamePlanPath)))
		if err != nil {
			return Revision{}, fmt.Errorf("read publication plan: %w", err)
		}
		if err = json.Unmarshal(data, &plan); err != nil {
			return Revision{}, fmt.Errorf("decode publication plan: %w", err)
		}
	}
	sceneBacked := project.Dimension == "3d" && sceneBackedCurrentAt(stage, &plan)
	if result.TargetedChecks {
		return Revision{}, fmt.Errorf("targeted check validation cannot be published")
	}
	if (project.Dimension == "2d" || sceneBacked) && (result.GameplayStatus != "passed" || !result.check.GameplayReceived) {
		return Revision{}, fmt.Errorf("publication requires full gameplay observations")
	}
	fingerprint, err := validationFingerprint(stage)
	if err != nil {
		return Revision{}, fmt.Errorf("verify publication files: %w", err)
	}
	if result.fingerprint == "" || result.fingerprint != fingerprint {
		return Revision{}, fmt.Errorf("project files changed after validation; validate the current project before publication")
	}
	if err := writeValidationReport(stage, result, s.opts.MaxFilesPerProject, s.opts.MaxFileBytes, s.opts.MaxProjectBytes); err != nil {
		return Revision{}, err
	}
	return s.publish(ctx, stage, project, job, "agent", job.Prompt)
}

func (s *Service) publish(ctx context.Context, stage string, project Project, job Job, source, summary string) (Revision, error) {
	if err := ctx.Err(); err != nil {
		return Revision{}, err
	}
	absStage, err := filepath.Abs(stage)
	if err != nil {
		return Revision{}, fmt.Errorf("resolve publication stage: %w", err)
	}
	stage = absStage
	files, total, err := s.snapshotFiles(stage)
	if err != nil {
		return Revision{}, err
	}
	published := false
	defer func() {
		if !published {
			_ = s.pruneOrphanBlobs(context.Background())
		}
	}()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Revision{}, fmt.Errorf("begin game maker revision: %w", err)
	}
	defer tx.Rollback()
	var currentRevision int64
	var projectKey string
	if err := tx.QueryRowContext(ctx, `SELECT current_revision,project_key FROM gm_projects WHERE id=?`, project.ID).Scan(&currentRevision, &projectKey); err != nil {
		return Revision{}, fmt.Errorf("read publication baseline: %w", err)
	}
	if currentRevision != project.CurrentRevision || projectKey != project.ProjectKey {
		return Revision{}, fmt.Errorf("publication revision conflict: refresh the project before publishing")
	}
	var next int64
	if err := tx.QueryRow(`SELECT COALESCE(MAX(number),0)+1 FROM gm_revisions WHERE project_id=?`, project.ID).Scan(&next); err != nil {
		return Revision{}, fmt.Errorf("allocate game maker revision: %w", err)
	}
	now := time.Now().UTC()
	result, err := tx.Exec(`INSERT INTO gm_revisions(project_id,number,parent,source,summary,file_count,total_bytes,created_at)
		VALUES(?,?,?,?,?,?,?,?)`, project.ID, next, project.CurrentRevision, source, summary, len(files), total, now)
	if err != nil {
		return Revision{}, fmt.Errorf("insert game maker revision: %w", err)
	}
	revisionID, _ := result.LastInsertId()
	for _, file := range files {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO gm_blobs(content_hash,size,created_at) VALUES(?,?,?)`,
			file.Hash, file.Size, now); err != nil {
			return Revision{}, fmt.Errorf("insert game maker blob: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO gm_revision_files(revision_id,path,content_hash,size) VALUES(?,?,?,?)`,
			revisionID, file.Path, file.Hash, file.Size); err != nil {
			return Revision{}, fmt.Errorf("insert game maker revision file: %w", err)
		}
	}
	if _, err := tx.Exec(`UPDATE gm_projects SET status='ready',current_revision=?,updated_at=? WHERE id=?`,
		next, now, project.ID); err != nil {
		return Revision{}, fmt.Errorf("update game maker published revision: %w", err)
	}
	receipt := filesystemReceipt{Version: 1, ID: randomID("publish"), Kind: "publish", ProjectID: project.ID, ProjectKey: project.ProjectKey,
		Stage: filepath.Base(stage), PreviousRevision: project.CurrentRevision, Revision: next}
	target, backup, expectedStage, err := s.receiptPaths(receipt)
	if err != nil {
		return Revision{}, err
	}
	if filepath.Clean(stage) != filepath.Clean(expectedStage) {
		return Revision{}, fmt.Errorf("publication stage is not owned by this service")
	}
	if _, err := os.Lstat(backup); !os.IsNotExist(err) {
		return Revision{}, fmt.Errorf("publication backup already exists or is unavailable")
	}
	if _, err := os.Stat(target); err == nil {
		receipt.HadTarget = true
	} else if !os.IsNotExist(err) {
		return Revision{}, err
	}
	receiptPath, err := s.writeFilesystemReceipt(receipt)
	if err != nil {
		return Revision{}, err
	}
	rollback := func(cause error) (Revision, error) {
		_ = tx.Rollback()
		recoveryErr := s.recoverFilesystemReceipt(context.Background(), receiptPath, receipt)
		return Revision{}, errors.Join(cause, recoveryErr)
	}
	if receipt.HadTarget {
		if err := fileutil.RenameContext(ctx, target, backup); err != nil {
			return rollback(fmt.Errorf("prepare game maker publication: %w", err))
		}
	}
	if err := fileutil.RenameContext(ctx, stage, target); err != nil {
		return rollback(fmt.Errorf("publish game maker project: %w", err))
	}
	if err := tx.Commit(); err != nil {
		// A commit error may be ambiguous. Recovery reads the durable outcome
		// before deciding which directory is authoritative.
		published = true
		_ = tx.Rollback()
		recoveryErr := s.recoverFilesystemReceipt(context.Background(), receiptPath, receipt)
		var durableRevision int64
		if recoveryErr == nil && s.db.QueryRowContext(context.Background(),
			`SELECT current_revision FROM gm_projects WHERE id=?`, project.ID).Scan(&durableRevision) == nil &&
			durableRevision == project.CurrentRevision {
			published = false
		}
		return Revision{}, errors.Join(fmt.Errorf("commit game maker revision: %w", err), recoveryErr)
	}
	published = true
	if err := s.recoverFilesystemReceipt(context.Background(), receiptPath, receipt); err != nil && s.opts.Logger != nil {
		s.opts.Logger.Warn("Game Maker publication cleanup retained for restart", "error", err)
	}
	return Revision{
		ID: revisionID, ProjectID: project.ID, Number: next, Parent: project.CurrentRevision,
		Source: source, Summary: summary, FileCount: len(files), TotalBytes: total, CreatedAt: now,
	}, nil
}

func (s *Service) pruneOrphanBlobs(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT b.content_hash
		FROM gm_blobs b
		LEFT JOIN gm_revision_files f ON f.content_hash=b.content_hash
		WHERE f.content_hash IS NULL`)
	if err != nil {
		return fmt.Errorf("list orphaned game maker blobs: %w", err)
	}
	hashSet := map[string]struct{}{}
	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan orphaned game maker blob: %w", err)
		}
		hashSet[hash] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("iterate orphaned game maker blobs: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close orphaned game maker blob rows: %w", err)
	}
	if err := filepath.WalkDir(s.blobDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		hash := entry.Name()
		if len(hash) != sha256.Size*2 {
			return nil
		}
		if _, err := hex.DecodeString(hash); err != nil {
			return nil
		}
		hashSet[hash] = struct{}{}
		return nil
	}); err != nil {
		return fmt.Errorf("scan game maker blob store: %w", err)
	}
	hashes := make([]string, 0, len(hashSet))
	for hash := range hashSet {
		hashes = append(hashes, hash)
	}
	sort.Strings(hashes)
	for _, hash := range hashes {
		if len(hash) != sha256.Size*2 {
			return fmt.Errorf("invalid game maker blob hash %q", hash)
		}
		if _, err := hex.DecodeString(hash); err != nil {
			return fmt.Errorf("invalid game maker blob hash %q: %w", hash, err)
		}
		var referenced bool
		if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(
			SELECT 1 FROM gm_revision_files WHERE content_hash=?)`, hash).Scan(&referenced); err != nil {
			return fmt.Errorf("check game maker blob references: %w", err)
		}
		if referenced {
			continue
		}
		blobPath := filepath.Join(s.blobDir, hash[:2], hash)
		if err := os.Remove(blobPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove orphaned game maker blob: %w", err)
		}
		if _, err := s.db.ExecContext(ctx, `DELETE FROM gm_blobs WHERE content_hash=?
			AND NOT EXISTS (SELECT 1 FROM gm_revision_files WHERE content_hash=?)`, hash, hash); err != nil {
			return fmt.Errorf("delete orphaned game maker blob record: %w", err)
		}
		_ = os.Remove(filepath.Dir(blobPath))
	}
	return nil
}

func (s *Service) snapshotFiles(root string) ([]revisionFile, int64, error) {
	var files []revisionFile
	var total int64
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: symlink in revision", ErrInvalidPath)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		hash := hex.EncodeToString(sum[:])
		blobPath := filepath.Join(s.blobDir, hash[:2], hash)
		if _, err := os.Stat(blobPath); os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(blobPath), 0o750); err != nil {
				return err
			}
			tmp, err := os.CreateTemp(filepath.Dir(blobPath), ".blob-*")
			if err != nil {
				return err
			}
			tmpName := tmp.Name()
			if _, err := tmp.Write(data); err != nil {
				_ = tmp.Close()
				_ = os.Remove(tmpName)
				return err
			}
			if err := tmp.Sync(); err != nil {
				_ = tmp.Close()
				_ = os.Remove(tmpName)
				return err
			}
			if err := tmp.Close(); err != nil {
				_ = os.Remove(tmpName)
				return err
			}
			if err := fileutil.Rename(tmpName, blobPath); err != nil && !errors.Is(err, os.ErrExist) {
				_ = os.Remove(tmpName)
				return err
			}
		} else if err != nil {
			return err
		}
		existing, err := os.ReadFile(blobPath)
		if err != nil {
			return fmt.Errorf("verify revision blob: %w", err)
		}
		existingHash := sha256.Sum256(existing)
		if len(existing) != len(data) || existingHash != sum {
			return fmt.Errorf("existing revision blob is corrupt")
		}
		if file, err := os.OpenFile(path, os.O_RDWR, 0); err != nil {
			return err
		} else {
			if err := file.Sync(); err != nil {
				file.Close()
				return fmt.Errorf("sync revision file: %w", err)
			}
			if err := file.Close(); err != nil {
				return err
			}
		}
		files = append(files, revisionFile{Path: filepath.ToSlash(rel), Hash: hash, Size: info.Size()})
		total += info.Size()
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, total, err
}

func (s *Service) RestoreRevision(ctx context.Context, projectID string, number int64) (Revision, error) {
	s.policyMu.RLock()
	defer s.policyMu.RUnlock()
	policy := s.policy
	if !policy.Enabled {
		return Revision{}, ErrDisabled
	}
	if policy.ReadOnly || !policy.AllowEdit {
		return Revision{}, ErrReadOnly
	}
	writerID := randomID("restore")
	if !s.reserveWriter(writerID) {
		return Revision{}, ErrBusy
	}
	defer s.releaseWriter(writerID)
	project, err := s.GetProject(ctx, projectID)
	if err != nil {
		return Revision{}, err
	}
	revision, err := scanRevision(s.db.QueryRowContext(ctx, `SELECT id,project_id,number,parent,source,summary,file_count,total_bytes,created_at
		FROM gm_revisions WHERE project_id=? AND number=?`, projectID, number))
	if errors.Is(err, sql.ErrNoRows) {
		return Revision{}, ErrNotFound
	}
	if err != nil {
		return Revision{}, fmt.Errorf("get game maker revision: %w", err)
	}
	stage := filepath.Join(s.stagingDir, randomID("restore"))
	if err := os.MkdirAll(stage, 0o750); err != nil {
		return Revision{}, err
	}
	removeStage := true
	defer func() {
		if removeStage {
			_ = os.RemoveAll(stage)
		}
	}()
	rows, err := s.db.QueryContext(ctx, `SELECT path,content_hash,size FROM gm_revision_files WHERE revision_id=? ORDER BY path`, revision.ID)
	if err != nil {
		return Revision{}, fmt.Errorf("list game maker revision files: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var file revisionFile
		if err := rows.Scan(&file.Path, &file.Hash, &file.Size); err != nil {
			return Revision{}, err
		}
		target, _, err := secureJoin(stage, file.Path, true)
		if err != nil {
			return Revision{}, err
		}
		data, err := os.ReadFile(filepath.Join(s.blobDir, file.Hash[:2], file.Hash))
		if err != nil {
			return Revision{}, fmt.Errorf("read game maker revision blob: %w", err)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != file.Hash {
			return Revision{}, fmt.Errorf("game maker revision blob hash mismatch")
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return Revision{}, err
		}
		if err := os.WriteFile(target, data, 0o640); err != nil {
			return Revision{}, err
		}
	}
	if err := rows.Err(); err != nil {
		return Revision{}, err
	}
	if result := buildDirectory(ctx, stage, s.opts.MaxFilesPerProject, s.opts.MaxProjectBytes); !result.OK {
		return Revision{}, fmt.Errorf("restored game revision failed validation: %s", diagnosticsText(result.Diagnostics))
	}
	restoreJob := Job{ID: writerID, ProjectID: projectID}
	published, err := s.publish(ctx, stage, project, restoreJob, "restore", fmt.Sprintf("Restored revision %d", number))
	if err != nil {
		removeStage = false
		return Revision{}, err
	}
	_, _ = s.emit(ctx, projectID, "", "revision", map[string]any{"revision": published, "restored_from": number})
	_, _ = s.emit(ctx, projectID, "", "preview_reload", map[string]any{"revision": published.Number})
	return published, nil
}
