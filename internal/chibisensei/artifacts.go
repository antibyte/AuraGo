package chibisensei

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"aurago/internal/security"
)

// Layout below <data>/chibi_sensei; every name is generated here, never taken from Kaggle (see the plan's table).
const (
	chibiDirName     = "chibi_sensei"
	snapshotSubdir   = "data"
	outSubdir        = "out"
	notebookFileName = "notebook.ipynb"
	logTailFileName  = "log-tail.txt"
	logTailBytes     = 64 << 10
	smallOutputMax   = 1 << 20 // status and manifest before their size is known
	minFreeDiskBytes = 64 << 20
)

var (
	errChecksumMismatch = errors.New("chibisensei: checksum mismatch")
	errOutputTooLarge   = errors.New("chibisensei: output larger than allowed")
	converterPattern    = regexp.MustCompile(`^[A-Za-z0-9._-]{1,40}$`)
)

func ChibiDir(dataDir string) string        { return filepath.Join(dataDir, chibiDirName) }
func RunDir(dataDir, runID string) string   { return filepath.Join(ChibiDir(dataDir), "runs", runID) }
func ModelDir(dataDir, runID string) string { return filepath.Join(ChibiDir(dataDir), "models", runID) }
func BaseDir(dataDir string) string         { return filepath.Join(ChibiDir(dataDir), "base") }
func snapshotDir(dataDir, runID string) string {
	return filepath.Join(RunDir(dataDir, runID), snapshotSubdir)
}

// runRel is a slash path below <data>/chibi_sensei inside the run directory.
func runRel(runID string, parts ...string) string {
	return path.Join(append([]string{"runs", runID}, parts...)...)
}

// stagedOutputName is the Kaggle project file a kernel output is downloaded into.
func stagedOutputName(runID, output string) string { return "r" + Run12(runID) + "-" + output }

// BaseGGUFConverter names the converter of a base GGUF (key of base_gguf): "llamacpp-<tag>".
func BaseGGUFConverter(llamaCppTag string) string { return "llamacpp-" + llamaCppTag }

func baseGGUFName(profileID, revision, converter string) (string, error) {
	if !profileIDPattern.MatchString(profileID) || !revisionPattern.MatchString(revision) || !converterPattern.MatchString(converter) {
		return "", errors.New("base gguf: invalid profile, revision or converter")
	}
	return profileID + "-" + revision[:12] + "-" + converter + ".gguf", nil
}

// outputRel is the storage location of a kernel output kind (and of manifest and log_tail).
func outputRel(r Run, f jobFacts, kind string) (string, error) {
	switch kind {
	case "status":
		return runRel(r.ID, outSubdir, "status.json"), nil
	case "manifest":
		return runRel(r.ID, outSubdir, "manifest.json"), nil
	case "metrics":
		return runRel(r.ID, outSubdir, "metrics.jsonl"), nil
	case "eval_samples":
		return runRel(r.ID, outSubdir, "eval_samples.jsonl"), nil
	case "log_tail":
		return runRel(r.ID, outSubdir, logTailFileName), nil
	case "gguf_tuned":
		return path.Join("models", r.ID, "model-q8_0.gguf"), nil
	case "adapter":
		return path.Join("models", r.ID, "adapter", "adapter_model.safetensors"), nil
	case "adapter_config":
		return path.Join("models", r.ID, "adapter", "adapter_config.json"), nil
	case "gguf_base":
		name, err := baseGGUFName(r.ProfileID, f.Profile.Base.Revision, f.converter())
		if err != nil {
			return "", err
		}
		return path.Join("base", name), nil
	}
	return "", fmt.Errorf("no storage location for output kind %q", kind)
}

func openChibiRoot(dataDir string) (*os.Root, error) {
	if err := os.MkdirAll(ChibiDir(dataDir), 0o700); err != nil {
		return nil, fmt.Errorf("create %s: %w", chibiDirName, err)
	}
	return os.OpenRoot(ChibiDir(dataDir))
}

// copyVerified copies src to rel below root through a .part file and publishes it only when the size and the
// sha256 match (wantBytes < 0 and wantSHA == "" accept any content up to limit). It returns the measured size and hash.
func copyVerified(root *os.Root, rel string, src io.Reader, wantBytes int64, wantSHA string, limit int64) (int64, string, error) {
	if err := root.MkdirAll(path.Dir(rel), 0o700); err != nil {
		return 0, "", fmt.Errorf("create %s: %w", path.Dir(rel), err)
	}
	part := rel + ".part"
	if err := removeQuiet(root, part); err != nil {
		return 0, "", err
	}
	f, err := root.OpenFile(part, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return 0, "", err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(src, limit+1))
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	sum := hex.EncodeToString(h.Sum(nil))
	switch {
	case err != nil:
	case n > limit:
		err = errOutputTooLarge
	case (wantBytes >= 0 && n != wantBytes) || (wantSHA != "" && sum != wantSHA):
		err = errChecksumMismatch
	}
	if err != nil {
		_ = removeQuiet(root, part)
		return n, sum, err
	}
	return n, sum, root.Rename(part, rel)
}

func writeChibiFile(dataDir, rel string, data []byte) error {
	root, err := openChibiRoot(dataDir)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.MkdirAll(path.Dir(rel), 0o700); err != nil {
		return err
	}
	if err := removeQuiet(root, rel+".part"); err != nil {
		return err
	}
	return writeRootFile(root, rel, data)
}

func readChibiFile(dataDir, rel string, limit int64) ([]byte, error) {
	root, err := os.OpenRoot(ChibiDir(dataDir))
	if err != nil {
		return nil, err
	}
	defer root.Close()
	f, err := root.Open(rel)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, errOutputTooLarge
	}
	return b, nil
}

// StorageBytes sums the regular files under <data>/chibi_sensei.
func StorageBytes(dataDir string) int64 {
	var total int64
	_ = filepath.WalkDir(ChibiDir(dataDir), func(_ string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return nil
		}
		if info, e := d.Info(); e == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

// RemoveRunFiles removes RunDir and ModelDir of a run; a missing directory is not an error; BaseDir is never touched.
func RemoveRunFiles(dataDir, runID string) error {
	if !ValidID(runID) {
		return errors.New("remove run files: invalid run id")
	}
	root, err := os.OpenRoot(ChibiDir(dataDir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer root.Close()
	var errs []error
	for _, rel := range []string{"runs/" + runID, "models/" + runID} {
		if err := root.RemoveAll(rel); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// logTailText turns a kernel log body into display text (a Kaggle JSON log joins its entries' data in order), scrubs
// it and keeps the last 64 KiB at a rune boundary.
func logTailText(body []byte) []byte {
	text := string(body)
	if entries, _ := decodeLogEntries(body); len(entries) > 0 {
		var b strings.Builder
		for _, e := range entries {
			b.WriteString(e.Data)
		}
		text = b.String()
	}
	if len(text) > 2*logTailBytes {
		text = text[len(text)-2*logTailBytes:]
	}
	text = strings.ToValidUTF8(security.Scrub(text), "")
	if len(text) > logTailBytes {
		cut := len(text) - logTailBytes
		for cut < len(text) && !utf8.RuneStart(text[cut]) {
			cut++
		}
		text = text[cut:]
	}
	return []byte(text)
}

func writeLogTail(dataDir, runID string, body []byte) error {
	return writeChibiFile(dataDir, runRel(runID, outSubdir, logTailFileName), logTailText(body))
}

// ReadLogTail returns the stored, scrubbed log tail of a run (nil, nil when there is none).
func ReadLogTail(dataDir, runID string) ([]byte, error) {
	if !ValidID(runID) {
		return nil, errors.New("read log tail: invalid run id")
	}
	b, err := readChibiFile(dataDir, runRel(runID, outSubdir, logTailFileName), 2*logTailBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return b, err
}

// OpenArtifact opens a registered artifact under <data>/chibi_sensei through an os.Root.
func OpenArtifact(dataDir string, a Artifact) (*os.File, error) {
	if !validArtifactPath(a.RelPath) {
		return nil, errors.New("open artifact: invalid path")
	}
	root, err := os.OpenRoot(ChibiDir(dataDir))
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.Open(a.RelPath)
}

// BaseGGUF is one shared base-model GGUF (table base_gguf).
type BaseGGUF struct {
	ProfileID, BaseRevision, Converter, RelPath, SHA256 string
}

func (s *Store) PutBaseGGUF(ctx context.Context, b BaseGGUF) error {
	if !validArtifactPath(b.RelPath) || !sha256Hex.MatchString(b.SHA256) {
		return errors.New("put base gguf: invalid path or hash")
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO base_gguf(profile_id,base_revision,converter,rel_path,sha256)
		VALUES(?,?,?,?,?) ON CONFLICT(profile_id,base_revision,converter) DO UPDATE SET rel_path=excluded.rel_path,
		sha256=excluded.sha256`, b.ProfileID, b.BaseRevision, b.Converter, b.RelPath, b.SHA256); err != nil {
		return fmt.Errorf("put base gguf: %w", err)
	}
	return nil
}

func (s *Store) GetBaseGGUF(ctx context.Context, profileID, revision, converter string) (BaseGGUF, bool, error) {
	b := BaseGGUF{ProfileID: profileID, BaseRevision: revision, Converter: converter}
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(rel_path,''), COALESCE(sha256,'') FROM base_gguf
		WHERE profile_id=? AND base_revision=? AND converter=?`, profileID, revision, converter).Scan(&b.RelPath, &b.SHA256)
	if errors.Is(err, sql.ErrNoRows) {
		return BaseGGUF{}, false, nil
	}
	if err != nil {
		return BaseGGUF{}, false, err
	}
	return b, true, nil
}

func (s *Store) deleteBaseGGUF(ctx context.Context, profileID, revision, converter string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM base_gguf WHERE profile_id=? AND base_revision=? AND converter=?`,
		profileID, revision, converter)
	return err
}

// otherRunsUse reports whether an artifact row of another run points at rel.
func (s *Store) otherRunsUse(ctx context.Context, runID, rel string) (bool, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM artifacts WHERE rel_path=? AND run_id<>?`, rel, runID).Scan(&n); err != nil {
		return false, fmt.Errorf("artifact references: %w", err)
	}
	return n > 0, nil
}

func (s *Store) deleteArtifact(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM artifacts WHERE id=?`, id)
	return err
}
