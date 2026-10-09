package localwiki

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"time"

	"aurago/internal/fileutil"
)

const (
	stateFileName    = "state.json"
	downloadFileName = "download.json"
	stateVersion     = 1
)

// sha256HexPattern is a lower-case hex SHA-256 digest.
var sha256HexPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// stateFile is <dir>/state.json: the installed edition and housekeeping.
type stateFile struct {
	Version         int       `json:"version"`
	Edition         *Edition  `json:"edition,omitempty"`
	LastUpdateCheck time.Time `json:"last_update_check,omitzero"`
	// Update is the newer edition the last update check found; it is kept so
	// the notice survives a restart until the next check.
	Update        *stateUpdate `json:"update,omitempty"`
	PendingDelete []string     `json:"pending_delete,omitempty"`
}

// stateUpdate names a newer edition of the installed language and variant.
type stateUpdate struct {
	Name string `json:"name"`
	Date string `json:"date"`
	Size int64  `json:"size"`
}

// downloadFile is <dir>/download.json: the edition a resumable download fetches.
type downloadFile struct {
	Version   int       `json:"version"`
	Target    Edition   `json:"target"`
	URLs      []string  `json:"urls"`
	LastURL   string    `json:"last_url,omitempty"`
	StartedAt time.Time `json:"started_at"`
}

// readState loads <dir>/state.json. A missing file, or one written by a newer
// AuraGo (unknown version, logged), reads as no state. The installed edition
// must name a file of the edition pattern and carry a plausible size and a
// SHA-256; pending deletes that are not bare edition file names are dropped.
func readState(dir string) (*stateFile, error) {
	var st stateFile
	found, err := readJSONFile(filepath.Join(dir, stateFileName), &st)
	if err != nil || !found {
		return nil, err
	}
	if st.Edition != nil {
		if err := validateEdition(*st.Edition); err != nil {
			return nil, fmt.Errorf("state.json: %w", err)
		}
	}
	st.Update = applicableUpdate(st.Edition, st.Update)
	st.PendingDelete = sanitizePendingDeletes(st.PendingDelete)
	return &st, nil
}

// applicableUpdate keeps a recorded update only while it still applies: the
// installed edition exists and the update is a newer edition of the same Kiwix
// language and variant. The date is taken from the edition name.
func applicableUpdate(installed *Edition, update *stateUpdate) *stateUpdate {
	if update == nil || installed == nil || update.Size < 0 || update.Size > maxEditionBytes {
		return nil
	}
	next, ok := parseEditionName(update.Name)
	current, okCurrent := parseEditionName(installed.Name)
	if !ok || !okCurrent || next.Kiwix != current.Kiwix || next.Variant != current.Variant || !editionNewer(update.Name, installed.Name) {
		return nil
	}
	return &stateUpdate{Name: update.Name, Date: next.Date, Size: update.Size}
}

func writeState(dir string, st *stateFile) error {
	st.Version = stateVersion
	return writeJSONFile(filepath.Join(dir, stateFileName), st)
}

// readDownload loads <dir>/download.json with the same version rule as
// readState. The target must be a valid edition and every mirror URL must pass
// the mirror rules of parseMeta4; trusted hosts (the catalog URL) may be local
// addresses. A last-used URL that fails them is logged and dropped; a file
// left without any mirror is refused.
func readDownload(dir string, trusted ...*url.URL) (*downloadFile, error) {
	var d downloadFile
	found, err := readJSONFile(filepath.Join(dir, downloadFileName), &d)
	if err != nil || !found {
		return nil, err
	}
	if err := validateEdition(d.Target); err != nil {
		return nil, fmt.Errorf("download.json: %w", err)
	}
	if len(d.URLs) > maxMirrors {
		return nil, fmt.Errorf("download.json lists %d mirrors, at most %d are allowed", len(d.URLs), maxMirrors)
	}
	for _, raw := range d.URLs {
		if err := checkMirrorURL(raw, d.Target.FileName, trusted); err != nil {
			return nil, fmt.Errorf("download.json: %w", err)
		}
	}
	// The last URL only reorders the mirrors; an invalid one is dropped
	// instead of discarding the resumable download.
	if d.LastURL != "" {
		if err := checkMirrorURL(d.LastURL, d.Target.FileName, trusted); err != nil {
			slog.Warn("[LocalWikipedia] Ignoring an invalid last mirror in download.json", "error", err)
			d.LastURL = ""
		}
	}
	// Without any mirror the download cannot be resumed; a fresh install
	// fetches the mirrors again.
	if len(d.URLs) == 0 && d.LastURL == "" {
		return nil, errors.New("download.json lists no mirror")
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

// validateEdition checks what a persisted edition record must satisfy before
// the manager trusts it: the on-disk file name pattern, a size in
// (0, maxEditionBytes] and a lower-case hex SHA-256.
func validateEdition(e Edition) error {
	if !zimFileNamePattern.MatchString(e.FileName) {
		return fmt.Errorf("names an unexpected file %q", e.FileName)
	}
	if e.Size <= 0 || e.Size > maxEditionBytes {
		return fmt.Errorf("%s has an implausible size of %d bytes", e.FileName, e.Size)
	}
	if !sha256HexPattern.MatchString(e.SHA256) {
		return fmt.Errorf("%s has no valid SHA-256", e.FileName)
	}
	return nil
}

// sanitizePendingDeletes keeps the entries that are bare edition file names
// (no separators, no "..") once each; the rest is logged and dropped so the
// manager can never be steered to delete anything else.
func sanitizePendingDeletes(names []string) []string {
	var kept []string
	for _, name := range names {
		if !zimFileNamePattern.MatchString(name) {
			slog.Warn("[LocalWikipedia] Ignoring an invalid pending delete in state.json", "entry", name)
			continue
		}
		if !slices.Contains(kept, name) {
			kept = append(kept, name)
		}
	}
	return kept
}

// readJSONFile reads path into target. found is false when the file does not
// exist or was written by a newer AuraGo: a version above stateVersion is
// logged and ignored without decoding the rest, whose layout may have changed.
func readJSONFile(path string, target any) (bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var head struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return false, fmt.Errorf("parse %s: %w", filepath.Base(path), err)
	}
	if head.Version < 0 {
		return false, fmt.Errorf("parse %s: invalid version %d", filepath.Base(path), head.Version)
	}
	if head.Version > stateVersion {
		slog.Warn("[LocalWikipedia] Ignoring a file written by a newer AuraGo", "file", filepath.Base(path), "version", head.Version)
		return false, nil
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
	if st.Update != nil {
		update := *st.Update
		out.Update = &update
	}
	out.PendingDelete = append([]string(nil), st.PendingDelete...)
	return &out
}
