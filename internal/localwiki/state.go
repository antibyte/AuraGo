package localwiki

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"aurago/internal/fileutil"
)

const (
	stateFileName    = "state.json"
	downloadFileName = "download.json"
	stateVersion     = 1
)

// stateFile is <dir>/state.json: the installed edition and housekeeping.
type stateFile struct {
	Version         int       `json:"version"`
	Edition         *Edition  `json:"edition,omitempty"`
	LastUpdateCheck time.Time `json:"last_update_check,omitzero"`
	PendingDelete   []string  `json:"pending_delete,omitempty"`
}

// downloadFile is <dir>/download.json: the edition a resumable download fetches.
type downloadFile struct {
	Version   int       `json:"version"`
	Target    Edition   `json:"target"`
	URLs      []string  `json:"urls"`
	LastURL   string    `json:"last_url,omitempty"`
	StartedAt time.Time `json:"started_at"`
}

func readState(dir string) (*stateFile, error) {
	var st stateFile
	found, err := readJSONFile(filepath.Join(dir, stateFileName), &st)
	if err != nil || !found {
		return nil, err
	}
	if st.Edition != nil && !zimFileNamePattern.MatchString(st.Edition.FileName) {
		return nil, fmt.Errorf("state.json names an unexpected file %q", st.Edition.FileName)
	}
	return &st, nil
}

func writeState(dir string, st *stateFile) error {
	st.Version = stateVersion
	return writeJSONFile(filepath.Join(dir, stateFileName), st)
}

func readDownload(dir string) (*downloadFile, error) {
	var d downloadFile
	found, err := readJSONFile(filepath.Join(dir, downloadFileName), &d)
	if err != nil || !found {
		return nil, err
	}
	if !zimFileNamePattern.MatchString(d.Target.FileName) {
		return nil, fmt.Errorf("download.json names an unexpected file %q", d.Target.FileName)
	}
	return &d, nil
}

func writeDownload(dir string, d *downloadFile) error {
	d.Version = stateVersion
	return writeJSONFile(filepath.Join(dir, downloadFileName), d)
}

func removeDownload(dir string) error {
	return removeIfExists(filepath.Join(dir, downloadFileName))
}

func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func readJSONFile(path string, target any) (bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(data, target); err != nil {
		return false, fmt.Errorf("parse %s: %w", filepath.Base(path), err)
	}
	return true, nil
}

func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return fileutil.WriteFileContext(context.Background(), path, append(data, '\n'), 0o644)
}

func cloneState(st *stateFile) *stateFile {
	if st == nil {
		return nil
	}
	out := *st
	if st.Edition != nil {
		edition := *st.Edition
		out.Edition = &edition
	}
	out.PendingDelete = append([]string(nil), st.PendingDelete...)
	return &out
}
