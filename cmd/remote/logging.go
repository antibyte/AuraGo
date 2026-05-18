package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const (
	defaultRemoteLogMaxBytes = 5 * 1024 * 1024
	defaultRemoteLogBackups  = 3
)

type rotatingFileWriter struct {
	mu         sync.Mutex
	path       string
	maxBytes   int64
	maxBackups int
	file       *os.File
}

func newRotatingFileWriter(path string, maxBytes int64, maxBackups int) (*rotatingFileWriter, error) {
	if path == "" {
		return nil, fmt.Errorf("log file path is required")
	}
	if maxBytes <= 0 {
		maxBytes = defaultRemoteLogMaxBytes
	}
	if maxBackups <= 0 {
		maxBackups = defaultRemoteLogBackups
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &rotatingFileWriter{
		path:       path,
		maxBytes:   maxBytes,
		maxBackups: maxBackups,
		file:       file,
	}, nil
}

func (w *rotatingFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		if err := w.openCurrentLocked(); err != nil {
			return 0, err
		}
	}
	info, err := w.file.Stat()
	if err != nil {
		return 0, err
	}
	if info.Size() > 0 && info.Size()+int64(len(p)) > w.maxBytes {
		if err := w.rotateLocked(); err != nil {
			return 0, err
		}
	}
	return w.file.Write(p)
}

func (w *rotatingFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

func (w *rotatingFileWriter) openCurrentLocked() error {
	file, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	w.file = file
	return nil
}

func (w *rotatingFileWriter) rotateLocked() error {
	if w.file != nil {
		if err := w.file.Close(); err != nil {
			return err
		}
		w.file = nil
	}

	_ = os.Remove(w.backupPath(w.maxBackups))
	for i := w.maxBackups - 1; i >= 1; i-- {
		src := w.backupPath(i)
		dst := w.backupPath(i + 1)
		if _, err := os.Stat(src); err == nil {
			_ = os.Rename(src, dst)
		}
	}
	if _, err := os.Stat(w.path); err == nil {
		_ = os.Rename(w.path, w.backupPath(1))
	}
	return w.openCurrentLocked()
}

func (w *rotatingFileWriter) backupPath(index int) string {
	return fmt.Sprintf("%s.%d", w.path, index)
}
