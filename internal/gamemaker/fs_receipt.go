package gamemaker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/config"
	"aurago/internal/fileutil"
)

// A synced receipt precedes any directory swap. Its private transaction ID
// owns exactly one backup; recovery never guesses from a directory suffix.
type filesystemReceipt struct {
	Version          int    `json:"version"`
	ID               string `json:"id"`
	Kind             string `json:"kind"`
	ProjectID        string `json:"project_id"`
	ProjectKey       string `json:"project_key"`
	Stage            string `json:"stage,omitempty"`
	PreviousRevision int64  `json:"previous_revision"`
	Revision         int64  `json:"revision"`
	HadTarget        bool   `json:"had_target"`
}

func (s *Service) receiptDir() string {
	return filepath.Join(filepath.Dir(s.opts.DBPath), "transactions")
}
func (s *Service) writeFilesystemReceipt(receipt filesystemReceipt) (string, error) {
	if _, _, _, err := s.receiptPaths(receipt); err != nil {
		return "", err
	}
	if err := os.MkdirAll(s.receiptDir(), 0700); err != nil {
		return "", err
	}
	body, err := json.Marshal(receipt)
	if err != nil {
		return "", err
	}
	path := filepath.Join(s.receiptDir(), receipt.ID+".json")
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return "", fmt.Errorf("filesystem receipt already exists or is unavailable")
	}
	if err := config.WriteFileAtomic(path, body, 0600); err != nil {
		return "", fmt.Errorf("persist filesystem receipt: %w", err)
	}
	return path, nil
}

func (s *Service) receiptPaths(r filesystemReceipt) (target, backup, stage string, err error) {
	if r.Version != 1 || r.ID == "" || strings.ContainsAny(r.ID, "/\\:.\x00") || r.ProjectID == "" ||
		!strings.HasPrefix(r.ProjectKey, "Games/") || filepath.ToSlash(filepath.Clean(r.ProjectKey)) != r.ProjectKey || strings.Contains(r.ProjectKey, "..") || strings.ContainsAny(r.ProjectKey, "\\:\x00") {
		return "", "", "", fmt.Errorf("invalid filesystem receipt identity")
	}
	if strings.Count(r.ProjectKey, "/") != 1 {
		return "", "", "", fmt.Errorf("invalid filesystem receipt project key")
	}
	workspaceRoot, err := filepath.Abs(s.opts.WorkspacePath)
	if err != nil {
		return "", "", "", err
	}
	target = filepath.Join(workspaceRoot, filepath.FromSlash(r.ProjectKey))
	if err := rejectSymlinkComponents(workspaceRoot, target); err != nil {
		return "", "", "", err
	}
	switch r.Kind {
	case "publish":
		if r.Stage == "" || filepath.Base(r.Stage) != r.Stage || strings.ContainsAny(r.Stage, "\\/:\x00") || r.Stage == "." || r.Stage == ".." || r.Revision <= r.PreviousRevision {
			return "", "", "", fmt.Errorf("invalid publication receipt")
		}
		stagingRoot, absErr := filepath.Abs(s.stagingDir)
		if absErr != nil {
			return "", "", "", absErr
		}
		stage = filepath.Join(stagingRoot, r.Stage)
		backup = target + ".gm-backup-" + r.ID
	case "delete":
		backup = target + ".gm-delete-" + r.ID
	default:
		return "", "", "", fmt.Errorf("unsupported filesystem receipt")
	}
	if err := rejectSymlinkComponents(workspaceRoot, backup); err != nil {
		return "", "", "", err
	}
	if stage != "" {
		if err := rejectSymlinkComponents(filepath.Dir(stage), stage); err != nil {
			return "", "", "", err
		}
	}
	return target, backup, stage, nil
}

func (s *Service) recoverFilesystemReceipts(ctx context.Context) error {
	entries, err := os.ReadDir(s.receiptDir())
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) > 1000 {
		return fmt.Errorf("too many game maker filesystem receipts")
	}
	for _, entry := range entries {
		path := filepath.Join(s.receiptDir(), entry.Name())
		if (strings.HasPrefix(entry.Name(), "publish_") || strings.HasPrefix(entry.Name(), "delete_")) && strings.Contains(entry.Name(), ".json.tmp-") {
			info, err := os.Lstat(path)
			if err != nil || !info.Mode().IsRegular() {
				return fmt.Errorf("invalid game maker transaction artifact")
			}
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("remove incomplete game maker receipt: %w", err)
			}
			continue
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			return fmt.Errorf("unrecognized game maker transaction artifact")
		}
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 16384 {
			return fmt.Errorf("invalid game maker transaction artifact")
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var receipt filesystemReceipt
		if err := json.Unmarshal(body, &receipt); err != nil {
			return fmt.Errorf("decode game maker receipt: %w", err)
		}
		if entry.Name() != receipt.ID+".json" {
			return fmt.Errorf("game maker receipt name mismatch")
		}
		if err := s.recoverFilesystemReceipt(ctx, path, receipt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) recoverFilesystemReceipt(ctx context.Context, path string, r filesystemReceipt) error {
	target, backup, stage, err := s.receiptPaths(r)
	if err != nil {
		return err
	}
	var key string
	var revision int64
	err = s.db.QueryRowContext(ctx, "SELECT project_key,current_revision FROM gm_projects WHERE id=?", r.ProjectID).Scan(&key, &revision)
	exists := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read publication outcome: %w", err)
	}
	if exists && key != r.ProjectKey {
		return fmt.Errorf("filesystem receipt project changed; review required")
	}
	committed := (r.Kind == "delete" && !exists) || (r.Kind == "publish" && exists && revision == r.Revision)
	if !committed && (!exists || revision != r.PreviousRevision) {
		return fmt.Errorf("ambiguous filesystem receipt; review required")
	}
	_, backupErr := os.Lstat(backup)
	if backupErr != nil && !os.IsNotExist(backupErr) {
		return backupErr
	}
	targetExists, err := receiptPathExists(target)
	if err != nil {
		return err
	}
	stageExists := false
	if r.Kind == "publish" {
		stageExists, err = receiptPathExists(stage)
		if err != nil {
			return err
		}
	}
	if committed {
		if r.Kind == "publish" {
			if !targetExists {
				return fmt.Errorf("committed publication missing; retain receipt")
			}
		}
		if backupErr == nil {
			if err := os.RemoveAll(backup); err != nil {
				return fmt.Errorf("cleanup committed transaction: %w", err)
			}
		}
	} else if backupErr == nil {
		if !r.HadTarget || (r.Kind == "publish" && !targetExists && !stageExists) {
			return fmt.Errorf("ambiguous filesystem receipt artifacts; review required")
		}
		if targetExists {
			if r.Kind != "publish" {
				return fmt.Errorf("deletion target reappeared; review required")
			}
			if stageExists {
				return fmt.Errorf("publication stage already exists; review required")
			}
			if err := fileutil.Rename(target, stage); err != nil {
				return err
			}
		}
		if err := fileutil.Rename(backup, target); err != nil {
			return err
		}
	} else if r.Kind == "publish" {
		if r.HadTarget {
			if !targetExists || !stageExists {
				return fmt.Errorf("publication artifacts missing; review required")
			}
		} else {
			if targetExists == stageExists {
				return fmt.Errorf("ambiguous publication artifacts; review required")
			}
			if targetExists {
				if err := fileutil.Rename(target, stage); err != nil {
					return err
				}
			}
		}
	} else if targetExists != r.HadTarget {
		return fmt.Errorf("deletion artifacts missing or unexpected; review required")
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove completed filesystem receipt: %w", err)
	}
	return nil
}

func receiptPathExists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
