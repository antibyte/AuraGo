package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"aurago/internal/config"
	"aurago/internal/ocr"
	"aurago/internal/security"
)

var publishedOCR atomic.Pointer[ocr.Manager]

func normalizeOCRInputPath(path string) string {
	return strings.TrimPrefix(filepath.ToSlash(path), "agent_workspace/workdir/")
}

// PublishOCR follows the existing server-owned optional-service publication pattern.
func PublishOCR(manager *ocr.Manager)  { publishedOCR.Store(manager) }
func WithdrawOCR(manager *ocr.Manager) { publishedOCR.CompareAndSwap(manager, nil) }
func OCRManager() *ocr.Manager         { return publishedOCR.Load() }

func stageOCRFile(ctx context.Context, cfg *config.Config, owner, path string) (ocr.File, error) {
	if ctx.Err() != nil {
		return ocr.File{}, ctx.Err()
	}
	m := OCRManager()
	if m == nil {
		return ocr.File{}, ocr.ErrNotReady
	}
	resolved, err := resolveToolInputPath(normalizeOCRInputPath(path), cfg)
	if err != nil {
		return ocr.File{}, err
	}
	source, err := rootedToolOpen(resolved)
	if err != nil {
		return ocr.File{}, err
	}
	defer source.Close()
	if info, err := source.Stat(); err != nil || !info.Mode().IsRegular() {
		return ocr.File{}, ocr.ErrInvalid
	}
	return m.Stage(owner, filepath.Base(path), source)
}

func ExecuteOCR(ctx context.Context, cfg *config.Config, session, operation, path, jobID, fileID, mode string) string {
	if cfg == nil || !cfg.OCR.Enabled || !cfg.OCR.AgentAccess {
		return ErrorJSONf("OCR agent access is disabled")
	}
	if session == "" {
		return ErrorJSONf("OCR requires an owned agent session")
	}
	m := OCRManager()
	if m == nil {
		return ErrorJSONf("OCR needs setup")
	}
	owner := "agent:" + session
	var result any
	var err error
	switch operation {
	case "extract":
		var file ocr.File
		file, err = stageOCRFile(ctx, cfg, owner, path)
		if err == nil {
			result, err = m.Submit(owner, []string{file.ID}, mode)
		}
	case "status":
		if jobID == "" {
			result = m.Status()
		} else {
			result, err = m.Get(owner, jobID)
		}
	case "result":
		var file ocr.File
		if fileID == "" {
			var job ocr.Job
			job, err = m.Get(owner, jobID)
			if err == nil && len(job.Files) == 1 {
				fileID = job.Files[0].ID
			}
		}
		if err == nil {
			file, err = m.Result(owner, jobID, fileID)
		}
		if err == nil {
			payload, _ := json.Marshal(file)
			result = map[string]any{"job_id": jobID, "file_id": fileID, "state": file.State, "content": security.IsolateExternalData(string(payload)),
				"attribution": ocr.Attribution, "license_url": ocr.ModelLicenseURL}
		}
	case "cancel":
		err = m.Cancel(owner, jobID)
		result = map[string]string{"job_id": jobID, "state": "cancelled"}
	default:
		err = ocr.ErrInvalid
	}
	if err != nil {
		return ErrorJSONf("OCR: %v", err)
	}
	// File display names are untrusted even in metadata-only status replies.
	if job, ok := result.(ocr.Job); ok {
		for i := range job.Files {
			job.Files[i].Name = security.IsolateExternalData(job.Files[i].Name)
		}
		result = job
	}
	encoded, _ := json.Marshal(map[string]any{"status": "success", "result": result})
	return string(encoded)
}

// ExtractOCRText is shared by the legacy PDF tool and the file indexer.
func ExtractOCRText(ctx context.Context, cfg *config.Config, path, owner string) (string, string, error) {
	m := OCRManager()
	if m == nil || cfg == nil || !cfg.OCR.Enabled || !m.Status().Ready {
		return "", "", ocr.ErrNotReady
	}
	file, err := stageOCRFile(ctx, cfg, owner, path)
	if err != nil {
		return "", "", err
	}
	job, err := m.SubmitAutomatic(owner, file.ID)
	if err != nil {
		return "", "", err
	}
	text, err := WaitOCRText(ctx, m, owner, job.ID)
	return text, job.ID, err
}
func WaitOCRText(ctx context.Context, m *ocr.Manager, owner, key string) (string, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		job, err := m.Get(owner, key)
		if err != nil {
			return "", err
		}
		if job.FinishedAt != nil {
			if job.State != "completed" {
				for _, file := range job.Files {
					if file.Error == ocr.ErrNotReady.Error() {
						return "", ocr.ErrNotReady
					}
				}
				return "", fmt.Errorf("OCR job %s", job.State)
			}
			var text strings.Builder
			for _, summary := range job.Files {
				file, err := m.Result(owner, key, summary.ID)
				if err != nil {
					return "", err
				}
				if file.State != "completed" {
					return "", errors.New("OCR result incomplete")
				}
				for _, page := range file.Pages {
					if page.Source == "error" {
						return "", errors.New("OCR page failed")
					}
					text.WriteString(page.Text)
					text.WriteByte('\n')
				}
			}
			return strings.TrimSpace(text.String()), nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
		}
	}
}

// OCRFingerprint invalidates only OCR-dependent indexing generations.
func OCRFingerprint(cfg *config.Config) string {
	if cfg == nil || !cfg.OCR.Enabled || !cfg.OCR.AutomaticIndexing {
		return ""
	}
	digest := sha256.Sum256([]byte(ocr.Version + "|" + ocr.ModelRevision + "|" + ocr.ReleaseImage + "|auto|200"))
	return "ocr=" + hex.EncodeToString(digest[:])
}
