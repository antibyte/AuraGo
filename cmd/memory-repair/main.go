// memory-repair merges proven duplicate analysis documents without embeddings.
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gofrs/flock"
	chromem "github.com/philippgille/chromem-go"
	_ "modernc.org/sqlite"
)

func openVectors(path string) (*chromem.Collection, error) {
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("existing vector directory required: %s", path)
	}
	if err := filepath.WalkDir(path, func(_ string, item os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if item.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("vector symlinks are unsupported")
		}
		return nil
	}); err != nil {
		return nil, err
	}
	db, err := chromem.NewPersistentDB(path, false)
	if err != nil {
		return nil, err
	}
	collection := db.GetCollection("aurago_memories", func(context.Context, string) ([]float32, error) {
		return nil, fmt.Errorf("embeddings are forbidden in offline repair")
	})
	if collection == nil {
		return nil, fmt.Errorf("aurago_memories collection is missing")
	}
	return collection, nil
}

func openSQLite(path string, writable bool) (*sql.DB, error) {
	mode := "ro"
	if writable {
		mode = "rw"
	}
	uriPath := strings.NewReplacer("%", "%25", "?", "%3F", "#", "%23").Replace(filepath.ToSlash(path))
	db, err := sql.Open("sqlite", "file:"+uriPath+"?mode="+mode)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA busy_timeout=5000`); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
func existingPath(path string, directory bool) (string, error) {
	if path == "" {
		return "", fmt.Errorf("all input paths are required")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(real)
	if err != nil {
		return "", err
	}
	if info.IsDir() != directory || (!directory && !info.Mode().IsRegular()) {
		return "", fmt.Errorf("invalid input path %s", path)
	}
	return real, nil
}
func artifactPath(root, path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, absolute)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("artifacts must stay under installation reports/")
	}
	for current := absolute; current != root; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("artifact symlinks are unsupported")
		}
	}
	return absolute, nil
}
func writeJSON(path string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(append(encoded, '\n'))
	syncErr := file.Sync()
	closeErr := file.Close()
	return errors.Join(writeErr, syncErr, closeErr)
}
func copyFile(from, to string) error {
	input, err := os.Open(from)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	syncErr := output.Sync()
	closeErr := output.Close()
	return errors.Join(copyErr, syncErr, closeErr)
}
func copyTree(ctx context.Context, from, to string) error {
	return filepath.WalkDir(from, func(path string, item os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if item.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("backup rejects symlinks")
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, rel)
		if item.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		if !item.Type().IsRegular() {
			return fmt.Errorf("backup rejects non-regular files")
		}
		return copyFile(path, target)
	})
}
func backupStores(ctx context.Context, db *sql.DB, vectors, root string) error {
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	path := filepath.Join(root, "memory.sqlite")
	if _, err := db.ExecContext(ctx, `VACUUM main INTO ?`, path); err != nil {
		return fmt.Errorf("consistent SQLite backup: %w", err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		return err
	}
	backup, err := openSQLite(path, false)
	if err != nil {
		return err
	}
	var integrity string
	err = backup.QueryRowContext(ctx, `PRAGMA quick_check`).Scan(&integrity)
	backup.Close()
	if err != nil || integrity != "ok" {
		return fmt.Errorf("SQLite backup integrity failed: %s: %w", integrity, err)
	}
	if err := copyTree(ctx, vectors, filepath.Join(root, "vectors")); err != nil {
		return err
	}
	manifest := map[string]string{}
	if err := filepath.WalkDir(root, func(path string, item os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if item.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, err = io.Copy(hash, file)
		file.Close()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		manifest[rel] = hex.EncodeToString(hash.Sum(nil))
		return nil
	}); err != nil {
		return err
	}
	return writeJSON(filepath.Join(root, "manifest.json"), manifest)
}

func run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("memory-repair", flag.ContinueOnError)
	dbArg := flags.String("db", "", "existing short_term.db")
	vectorArg := flags.String("vector-db", "", "existing Chromem directory")
	installArg := flags.String("install-dir", "", "actual directory containing aurago and aurago.lock")
	planArg := flags.String("plan", "", "preview JSON under installation reports/; mandatory with --apply")
	apply := flags.Bool("apply", false, "apply a saved plan after backup and rehearsal")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	if *apply && *planArg == "" {
		return fmt.Errorf("--apply requires a saved --plan")
	}
	database, err := existingPath(*dbArg, false)
	if err != nil {
		return fmt.Errorf("resolve memory database: %w", err)
	}
	vectors, err := existingPath(*vectorArg, true)
	if err != nil {
		return fmt.Errorf("resolve vector directory: %w", err)
	}
	install, err := existingPath(*installArg, true)
	if err != nil {
		return fmt.Errorf("resolve installation directory: %w", err)
	}
	lockPath := filepath.Join(install, "aurago.lock")
	if info, err := os.Lstat(lockPath); err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("existing application lock required; verify --install-dir")
	}
	lock := flock.New(lockPath)
	held, err := lock.TryLock()
	if err != nil {
		return fmt.Errorf("acquire application lock: %w", err)
	}
	if !held {
		return fmt.Errorf("AuraGo is running; stop the agent before offline memory repair")
	}
	defer lock.Unlock()
	reports := filepath.Join(install, "reports")
	if relative, err := filepath.Rel(vectors, reports); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("reports must be outside the vector store to avoid recursive backups")
	}
	if info, err := os.Lstat(reports); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("reports directory must not be a symlink")
	}
	if err := os.MkdirAll(reports, 0700); err != nil {
		return err
	}
	runID := time.Now().UTC().Format("20060102T150405Z") + "-" + rand.Text()
	planPath := *planArg
	if planPath == "" {
		planPath = filepath.Join(reports, "memory-merge-"+runID+"-preview.json")
	}
	planPath, err = artifactPath(reports, planPath)
	if err != nil {
		return err
	}
	db, err := openSQLite(database, *apply)
	if err != nil {
		return err
	}
	defer db.Close()
	if !*apply {
		collection, err := openVectors(vectors)
		if err != nil {
			return err
		}
		plan, err := buildPlan(ctx, db, collection)
		if err != nil {
			return err
		}
		plan.Database, plan.Vectors, plan.InstallDir = database, vectors, install
		if err := writeJSON(planPath, plan); err != nil {
			return fmt.Errorf("save merge preview: %w", err)
		}
		fmt.Printf("Preview: %d merge groups, %d review items; %s\n", len(plan.Groups), len(plan.Review), planPath)
		return nil
	}
	input, err := os.Open(planPath)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(input)
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	var plan mergePlan
	err = decoder.Decode(&plan)
	input.Close()
	if err != nil {
		return err
	}
	if plan.Version != 1 || plan.Database != database || plan.Vectors != vectors || plan.InstallDir != install {
		return fmt.Errorf("plan belongs to different inputs or version")
	}
	work := filepath.Join(reports, "memory-merge-"+runID)
	if err := backupStores(ctx, db, vectors, filepath.Join(work, "backup")); err != nil {
		return err
	}
	if err := copyTree(ctx, filepath.Join(work, "backup"), filepath.Join(work, "rehearsal")); err != nil {
		return err
	}
	rehearsal, err := openSQLite(filepath.Join(work, "rehearsal", "memory.sqlite"), true)
	if err != nil {
		return err
	}
	_, failed := applyGroups(ctx, rehearsal, filepath.Join(work, "rehearsal", "vectors"), plan, nil)
	approved := plan
	approved.Groups = nil
	for _, group := range plan.Groups {
		blocked := false
		for _, failure := range failed {
			if fingerprint(failure.IDs) == fingerprint(group.IDs) {
				blocked = true
				break
			}
		}
		if !blocked {
			approved.Groups = append(approved.Groups, group)
		}
	}
	repeated, retryFailures := applyGroups(ctx, rehearsal, filepath.Join(work, "rehearsal", "vectors"), approved, nil)
	var integrity string
	integrityErr := rehearsal.QueryRowContext(ctx, `PRAGMA quick_check`).Scan(&integrity)
	rehearsal.Close()
	if repeated != 0 || len(retryFailures) != 0 || integrityErr != nil || integrity != "ok" {
		return fmt.Errorf("rehearsal was not idempotent or consistent")
	}
	if err := writeJSON(filepath.Join(work, "rehearsal-result.json"), map[string]any{"approved_groups": len(approved.Groups), "review_required": append(append([]reviewItem{}, plan.Review...), failed...)}); err != nil {
		return err
	}
	completed, liveFailures := applyGroups(ctx, db, vectors, approved, nil)
	failed = append(failed, liveFailures...)
	remaining := append(append([]reviewItem{}, plan.Review...), failed...)
	if err := writeJSON(filepath.Join(work, "result.json"), map[string]any{"completed_groups": completed, "review_required": remaining, "plan": planPath}); err != nil {
		return err
	}
	fmt.Printf("Applied: %d groups; %d incomplete groups; %d review items; evidence: %s\n", completed, len(failed), len(remaining), work)
	if len(failed) > 0 {
		return fmt.Errorf("some groups remain unchanged or pending; inspect result.json")
	}
	return nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Memory merge failed:", err)
		os.Exit(1)
	}
}
