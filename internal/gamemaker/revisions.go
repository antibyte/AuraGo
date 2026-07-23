package gamemaker

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
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

func (s *Service) publish(stage string, project Project, job Job, source, summary string) (Revision, error) {
	files, total, err := s.snapshotFiles(stage)
	if err != nil {
		return Revision{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Revision{}, fmt.Errorf("begin game maker revision: %w", err)
	}
	defer tx.Rollback()
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
	target := filepath.Join(s.opts.WorkspacePath, filepath.FromSlash(project.ProjectKey))
	backup := target + ".gm-backup-" + job.ID
	_ = os.RemoveAll(backup)
	hadTarget := false
	if _, err := os.Stat(target); err == nil {
		hadTarget = true
		if err := os.Rename(target, backup); err != nil {
			return Revision{}, fmt.Errorf("prepare game maker publication: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return Revision{}, fmt.Errorf("inspect game maker publication target: %w", err)
	}
	if err := os.Rename(stage, target); err != nil {
		if hadTarget {
			_ = os.Rename(backup, target)
		}
		return Revision{}, fmt.Errorf("publish game maker project: %w", err)
	}
	if err := tx.Commit(); err != nil {
		_ = os.RemoveAll(target)
		if hadTarget {
			_ = os.Rename(backup, target)
		}
		return Revision{}, fmt.Errorf("commit game maker revision: %w", err)
	}
	_ = os.RemoveAll(backup)
	return Revision{
		ID: revisionID, ProjectID: project.ID, Number: next, Parent: project.CurrentRevision,
		Source: source, Summary: summary, FileCount: len(files), TotalBytes: total, CreatedAt: now,
	}, nil
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
			if err := tmp.Close(); err != nil {
				_ = os.Remove(tmpName)
				return err
			}
			if err := os.Rename(tmpName, blobPath); err != nil && !errors.Is(err, os.ErrExist) {
				_ = os.Remove(tmpName)
				return err
			}
		} else if err != nil {
			return err
		}
		files = append(files, revisionFile{Path: filepath.ToSlash(rel), Hash: hash, Size: info.Size()})
		total += info.Size()
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, total, err
}

func (s *Service) RestoreRevision(ctx context.Context, projectID string, number int64) (Revision, error) {
	if !s.opts.Enabled {
		return Revision{}, ErrDisabled
	}
	if s.opts.ReadOnly || !s.opts.AllowEdit {
		return Revision{}, ErrReadOnly
	}
	s.mu.Lock()
	if s.activeJobID != "" {
		s.mu.Unlock()
		return Revision{}, ErrBusy
	}
	s.mu.Unlock()
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
	defer os.RemoveAll(stage)
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
	restoreJob := Job{ID: randomID("restore"), ProjectID: projectID}
	published, err := s.publish(stage, project, restoreJob, "restore", fmt.Sprintf("Restored revision %d", number))
	if err != nil {
		return Revision{}, err
	}
	_, _ = s.emit(ctx, projectID, "", "revision", map[string]any{"revision": published, "restored_from": number})
	_, _ = s.emit(ctx, projectID, "", "preview_reload", map[string]any{"revision": published.Number})
	return published, nil
}
