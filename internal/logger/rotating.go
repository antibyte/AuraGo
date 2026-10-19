package logger

import (
	"fmt"
	"os"
	"sync"
)

const defaultLogBytes int64 = 10 << 20
const defaultLogBackups = 3

type rotatingLog struct {
	mu          sync.Mutex
	file        *os.File
	path        string
	size, limit int64
	backups     int
	regular     bool
}

func newRotatingLog(path string, appendMode bool, limit int64, backups int) (*rotatingLog, error) {
	file, err := openLogFile(path, appendMode)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	return &rotatingLog{file: file, path: path, size: info.Size(), limit: limit, backups: backups, regular: info.Mode().IsRegular()}, nil
}
func (w *rotatingLog) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.regular && w.limit > 0 && w.size+int64(len(p)) > w.limit {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	if w.regular && w.limit > 0 && int64(len(p)) > w.limit {
		// Keep oversized structured records valid and bound disk usage per file.
		original := len(p)
		p = []byte("[log record omitted: exceeds file size limit]\n")
		_, err := w.file.Write(p)
		w.size += int64(len(p))
		if err != nil {
			return 0, err
		}
		return original, nil
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}
func (w *rotatingLog) rotate() error {
	if err := w.file.Close(); err != nil {
		return err
	}
	for i := w.backups; i >= 1; i-- {
		target := fmt.Sprintf("%s.%d", w.path, i)
		if i == w.backups {
			if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
				return w.reopenAfterFailure(err)
			}
		}
		source := w.path
		if i > 1 {
			source = fmt.Sprintf("%s.%d", w.path, i-1)
		}
		if err := os.Rename(source, target); err != nil && !os.IsNotExist(err) {
			return w.reopenAfterFailure(err)
		}
	}
	file, err := openLogFile(w.path, false)
	if err != nil {
		return err
	}
	w.file = file
	w.size = 0
	return nil
}
func (w *rotatingLog) reopenAfterFailure(cause error) error {
	file, err := openLogFile(w.path, true)
	if err == nil {
		w.file = file
	}
	return fmt.Errorf("rotate log: %w", cause)
}
func (w *rotatingLog) Close() error { w.mu.Lock(); defer w.mu.Unlock(); return w.file.Close() }
