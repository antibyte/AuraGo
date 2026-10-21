package services

import (
	"aurago/internal/ocr"
	"aurago/internal/tools"
	"context"
	"errors"
	"os"
	"path/filepath"
)

func (fi *FileIndexer) extractOCR(ctx context.Context, dir, path string) (string, bool, error) {
	fi.cfgMu.RLock()
	cfg := *fi.cfg
	fi.cfgMu.RUnlock()
	if !cfg.OCR.Enabled || !cfg.OCR.AutomaticIndexing || !ocr.Supported(path) {
		return "", false, nil
	}
	manager := tools.OCRManager()
	if manager == nil || !manager.Status().Ready {
		return "", false, nil
	}
	ctx, cancel := context.WithTimeout(ctx, ocr.FileTimeout)
	defer cancel()
	root, err := os.OpenRoot(dir)
	if err != nil {
		return "", true, err
	}
	defer root.Close()
	relative, err := filepath.Rel(dir, path)
	if err != nil {
		return "", true, err
	}
	source, err := root.Open(relative)
	if err != nil {
		return "", true, err
	}
	defer source.Close()
	owner := "indexer:" + path
	file, err := manager.Stage(owner, filepath.Base(path), source)
	if err != nil {
		return "", true, err
	}
	job, err := manager.SubmitAutomatic(owner, file.ID)
	if err != nil {
		return "", true, err
	}
	text, err := tools.WaitOCRText(ctx, manager, owner, job.ID)
	if errors.Is(err, ocr.ErrNotReady) {
		return "", false, nil
	}
	return text, true, err
}
