package flows

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"aurago/internal/security"
)

// writePurgeBackup copies flows.db with VACUUM INTO into a temporary file in dir, encrypts it into
// .flows-purge-backup-<UTC>.enc and removes the plaintext copy. It returns the file name.
func (s *Store) writePurgeBackup(ctx context.Context, dir, key string, now time.Time) (string, error) {
	suffix := make([]byte, 6)
	if _, err := rand.Read(suffix); err != nil {
		return "", err
	}
	tmp := filepath.Join(dir, purgeTempPrefix+hex.EncodeToString(suffix))
	defer os.Remove(tmp)
	if _, err := s.db.ExecContext(ctx, `VACUUM INTO ?`, tmp); err != nil {
		return "", fmt.Errorf("copy flows.db: %w", err)
	}
	_ = os.Chmod(tmp, 0o600)
	name := PurgeBackupPrefix + now.UTC().Format(backupTimeLayout) + PurgeBackupSuffix
	if err := security.EncryptFileWithMasterKey(tmp, filepath.Join(dir, name), PurgeBackupLabel, key); err != nil {
		return "", fmt.Errorf("encrypt the backup: %w", err)
	}
	return name, nil
}

func replaceText(text string, literals []string) string {
	for _, lit := range literals {
		text = strings.ReplaceAll(text, lit, CredentialRedactionMarker)
	}
	return text
}

// replaceLiterals replaces every exact occurrence of a literal inside the strings (and keys) of a decoded value.
func replaceLiterals(v any, literals []string) any {
	switch x := v.(type) {
	case string:
		return replaceText(x, literals)
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = replaceLiterals(x[i], literals)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, item := range x {
			out[replaceText(k, literals)] = replaceLiterals(item, literals)
		}
		return out
	}
	return v
}

// redactStoredText blacks out what the rule finds in JSON text and every exact occurrence of a literal.
// Text that is not JSON (a truncated output) only gets the exact replacement. It reports whether the
// text changed; running it on its own result changes nothing.
func redactStoredText(text string, literals []string) (string, bool) {
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	var v any
	valid := dec.Decode(&v) == nil
	if valid {
		_, err := dec.Token()
		valid = errors.Is(err, io.EOF)
	}
	if !valid {
		out := replaceText(text, literals)
		return out, out != text
	}
	redacted := replaceLiterals(redactCredentialValue(v), literals)
	if reflect.DeepEqual(v, redacted) {
		return text, false
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(redacted); err != nil {
		return text, false
	}
	return strings.TrimRight(buf.String(), "\n"), true
}

// redactStoredRows rewrites the plaintext JSON columns in batches and returns the number of rewritten cells.
func (s *Store) redactStoredRows(ctx context.Context, conn *sql.Conn, literals []string) (int, error) {
	total := 0
	for _, src := range credentialTextSources {
		for _, column := range src.columns {
			n, err := redactColumn(ctx, conn, src.table, column, literals)
			total += n
			if err != nil {
				return total, fmt.Errorf("%s.%s: %w", src.table, column, err)
			}
		}
	}
	return total, nil
}

func redactColumn(ctx context.Context, conn *sql.Conn, table, column string, literals []string) (int, error) {
	const batchSize = 200
	changed, last := 0, int64(0)
	type update struct {
		id   int64
		text string
	}
	for {
		rows, err := conn.QueryContext(ctx, `SELECT rowid, `+column+` FROM `+table+` WHERE rowid > ? AND `+column+` <> '' ORDER BY rowid LIMIT `+fmt.Sprint(batchSize), last)
		if err != nil {
			return changed, err
		}
		var batch []update
		n := 0
		for rows.Next() {
			var id int64
			var text string
			if err := rows.Scan(&id, &text); err != nil {
				_ = rows.Close()
				return changed, err
			}
			n++
			last = id
			if out, ok := redactStoredText(text, literals); ok {
				batch = append(batch, update{id, out})
			}
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return changed, err
		}
		if len(batch) > 0 {
			tx, err := conn.BeginTx(ctx, nil)
			if err != nil {
				return changed, err
			}
			for _, u := range batch {
				if _, err := tx.ExecContext(ctx, `UPDATE `+table+` SET `+column+` = ? WHERE rowid = ?`, u.text, u.id); err != nil {
					_ = tx.Rollback()
					return changed, err
				}
			}
			if err := tx.Commit(); err != nil {
				return changed, err
			}
			changed += len(batch)
		}
		if n < batchSize {
			return changed, nil
		}
	}
}

// compactDatabase runs VACUUM and truncates the WAL, on a connection that has secure_delete on.
func compactDatabase(ctx context.Context, conn *sql.Conn) error {
	if _, err := conn.ExecContext(ctx, `VACUUM`); err != nil {
		return fmt.Errorf("vacuum: %w", err)
	}
	for attempt := 0; attempt < 10; attempt++ {
		var busy, logFrames, checkpointed int
		if err := conn.QueryRowContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &logFrames, &checkpointed); err != nil {
			return fmt.Errorf("checkpoint: %w", err)
		}
		if busy == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}
	return errors.New("the WAL could not be truncated because the database stayed busy")
}

// deleteSnapshots removes the chosen snapshot directories (names are validated by validateSnapshotNames).
func deleteSnapshots(dir string, names []string) ([]string, error) {
	deleted := []string{}
	for _, name := range names {
		if !snapshotNamePattern.MatchString(name) {
			return deleted, ErrPurgeSnapshot
		}
		path := filepath.Join(dir, name)
		if info, err := os.Lstat(path); err != nil || !info.IsDir() {
			continue // already gone: running again is safe
		}
		if err := os.RemoveAll(path); err != nil {
			return deleted, err
		}
		deleted = append(deleted, name)
	}
	return deleted, nil
}

// fileHasAny reports whether the file contains one of the needles (a missing file does not).
func fileHasAny(path string, needles [][]byte) (bool, error) {
	if len(needles) == 0 {
		return false, nil
	}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	maxLen := 0
	for _, n := range needles {
		if len(n) > maxLen {
			maxLen = len(n)
		}
	}
	buf := make([]byte, 1<<20+maxLen)
	carry := 0
	for {
		n, err := f.Read(buf[carry:])
		window := buf[:carry+n]
		for _, needle := range needles {
			if bytes.Contains(window, needle) {
				return true, nil
			}
		}
		if errors.Is(err, io.EOF) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		keep := maxLen - 1
		if keep > len(window) {
			keep = len(window)
		}
		copy(buf, window[len(window)-keep:])
		carry = keep
	}
}

// verifyPurge scans again and searches the files byte for byte. A literal that a kept document still
// needs (a draft or live document that was not moved, a version a run still pins) is not searched for
// in the files; it is still counted in the run, step and test-data rows, which must be clean.
func (s *Store) verifyPurge(ctx context.Context, dbPath, dir string, literals []string) ([]PurgeRemaining, error) {
	post, err := s.scanCredentials(ctx)
	if err != nil {
		return nil, err
	}
	remaining := []PurgeRemaining{}
	for _, item := range []PurgeRemaining{{"runs", post.report.Runs}, {"steps", post.report.Steps}, {"test_data", post.report.TestRows}} {
		if item.Count > 0 {
			remaining = append(remaining, item)
		}
	}
	var needles [][]byte
	// keep is expanded like the purge set, so the bare part of a kept "Bearer ..." value is kept too.
	keep := map[string]struct{}{}
	for _, lit := range expandLiterals(post.draftLive, post.versions) {
		keep[lit] = struct{}{}
	}
	for _, lit := range literals {
		if _, kept := keep[lit]; !kept {
			needles = append(needles, []byte(lit))
		}
	}
	files := []string{dbPath, dbPath + "-wal", dbPath + "-shm"}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.Type().IsRegular() && strings.HasPrefix(name, ".flows-") && !strings.HasSuffix(name, PurgeBackupSuffix) {
			files = append(files, filepath.Join(dir, name))
		}
	}
	inside, _ := filepath.Glob(filepath.Join(dir, ".flows-v*-backup-*", "*"))
	files = append(files, inside...)
	for _, file := range files {
		hit, err := fileHasAny(file, needles)
		if err != nil {
			return nil, err
		}
		if hit {
			rel, relErr := filepath.Rel(dir, file)
			if relErr != nil {
				rel = filepath.Base(file)
			}
			remaining = append(remaining, PurgeRemaining{Where: filepath.ToSlash(rel), Count: 1})
		}
	}
	return remaining, nil
}
