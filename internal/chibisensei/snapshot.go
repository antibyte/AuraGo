package chibisensei

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"strings"
)

// SnapshotSplits is the order of the split files in the marker, the data hash and Snapshot.Files.
var SnapshotSplits = []string{"train", "val", "eval"}

type SnapshotFile struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Rows   int    `json:"rows"`
	Bytes  int64  `json:"bytes"`
}

type Snapshot struct {
	RunID   string
	Dir     string
	Files   map[string]SnapshotFile // keys train, val, eval; a split with zero rows is absent
	Marker  SnapshotFile            // Rows is 0
	Format  string                  // messages | text
	DataRev int64
	DataSHA string // sha256 over "<split>:<sha256>\n" for the present splits in the order train, val, eval
	Bytes   int64  // sum of all files
}

type snapshotMarker struct {
	V       int                     `json:"v"`
	RunID   string                  `json:"run_id"`
	Format  string                  `json:"format"`
	DataRev int64                   `json:"data_rev"`
	Files   map[string]SnapshotFile `json:"files"`
}

// freezeAfterLoad is a test seam called after the split rows are loaded; nil in production.
var freezeAfterLoad func()

// Run12 is the first 12 hex characters of an ID with the dashes removed.
func Run12(runID string) string {
	h := strings.ToLower(strings.ReplaceAll(runID, "-", ""))
	if len(h) > 12 {
		h = h[:12]
	}
	return h
}

// SnapshotFileName is the staged and dataset file name of one split (D12).
func SnapshotFileName(runID, split string) string { return "r" + Run12(runID) + "-" + split + ".jsonl" }

// MarkerFileName is the staged and dataset file name of the marker; job.json names it in data.marker.
func MarkerFileName(runID string) string { return "r" + Run12(runID) + "-marker.json" }

// DatasetSlug is the Kaggle dataset slug of a run.
func DatasetSlug(runID string) string { return "chibi-data-" + Run12(runID) }

// NotebookSlug is the Kaggle notebook slug: chibi-<first 8 hex of the project id>-<run12>.
func NotebookSlug(projectID, runID string) string {
	p := strings.ToLower(strings.ReplaceAll(projectID, "-", ""))
	if len(p) > 8 {
		p = p[:8]
	}
	return "chibi-" + p + "-" + Run12(runID)
}

// FormatFor names the data format of a recipe: text for free, messages otherwise ("" when unknown).
func FormatFor(recipe string) string {
	switch recipe {
	case RecipeAssistant, RecipeSpecialist:
		return "messages"
	case RecipeFree:
		return "text"
	}
	return ""
}

// marshalStable encodes v in struct order without HTML escaping, indented by one space, with one trailing newline.
func marshalStable(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type splitFile struct {
	name string
	f    *os.File
	h    hash.Hash
	bw   *bufio.Writer
	rows int
	n    int64
}

// FreezeSnapshot writes the split files and the marker into dir and returns their facts. It fails with
// ErrDataChanged when the rows changed while it read them. Excluded rows are skipped; a split with zero rows
// gets no file (D11). Older files of the same run are replaced; on failure none of the run's files remain.
func (s *Store) FreezeSnapshot(ctx context.Context, projectID, runID, dir string) (Snapshot, error) {
	if !ValidID(runID) {
		return Snapshot{}, fmt.Errorf("snapshot: invalid run id")
	}
	srows, proj, err := s.LoadSplitRows(ctx, projectID)
	if err != nil {
		return Snapshot{}, fmt.Errorf("snapshot: load rows: %w", err)
	}
	if freezeAfterLoad != nil {
		freezeAfterLoad()
	}
	format := FormatFor(proj.Recipe)
	if format == "" {
		return Snapshot{}, fmt.Errorf("snapshot: unknown recipe %q", proj.Recipe)
	}
	sp := ComputeSplit(srows, proj.SplitSeed, proj.TaskKind == "classify")
	if sp.Counts.Train == 0 {
		return Snapshot{}, &ValidationError{Issues: []Issue{issue("rows", "snapshot.no_train_rows", "The project has no training rows.")}}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Snapshot{}, fmt.Errorf("snapshot: create directory: %w", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return Snapshot{}, fmt.Errorf("snapshot: open directory: %w", err)
	}
	defer root.Close()
	names := []string{MarkerFileName(runID)}
	for _, split := range SnapshotSplits {
		names = append(names, SnapshotFileName(runID, split))
	}
	cleanup := func() error {
		for _, n := range names {
			if err := removeQuiet(root, n); err != nil {
				return err
			}
			if err := removeQuiet(root, n+".part"); err != nil {
				return err
			}
		}
		return nil
	}
	if err := cleanup(); err != nil {
		return Snapshot{}, fmt.Errorf("snapshot: remove old files: %w", err)
	}
	snap, err := s.writeSnapshot(ctx, root, proj, sp, runID, format)
	if err != nil {
		_ = cleanup()
		return Snapshot{}, err
	}
	snap.Dir = dir
	return snap, nil
}

func (s *Store) writeSnapshot(ctx context.Context, root *os.Root, proj Project, sp SplitResult, runID, format string) (Snapshot, error) {
	want := map[string]int{"train": sp.Counts.Train, "val": sp.Counts.Val, "eval": sp.Counts.Eval}
	files := map[string]*splitFile{}
	defer func() {
		for _, sf := range files {
			if sf.f != nil {
				sf.f.Close()
			}
		}
	}()
	for _, split := range SnapshotSplits {
		if want[split] == 0 {
			continue
		}
		name := SnapshotFileName(runID, split)
		fh, err := root.OpenFile(name+".part", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return Snapshot{}, fmt.Errorf("snapshot: create %s: %w", name, err)
		}
		sf := &splitFile{name: name, f: fh, h: sha256.New()}
		sf.bw = bufio.NewWriterSize(io.MultiWriter(fh, sf.h), 64<<10)
		files[split] = sf
	}
	err := s.EachRow(ctx, proj.ID, func(r Row) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		sf := files[sp.Assign[r.ID]] // excluded rows and rows added after the load have no file
		if sf == nil {
			return nil
		}
		line := ExportLine(r)
		sf.rows++
		sf.n += int64(len(line))
		_, err := sf.bw.Write(line)
		return err
	})
	if err != nil {
		return Snapshot{}, fmt.Errorf("snapshot: write rows: %w", err)
	}
	now, err := s.GetProject(ctx, proj.ID)
	if err != nil {
		return Snapshot{}, fmt.Errorf("snapshot: recheck: %w", err)
	}
	if now.DataRev != proj.DataRev {
		return Snapshot{}, ErrDataChanged
	}
	snap := Snapshot{RunID: runID, Files: map[string]SnapshotFile{}, Format: format, DataRev: proj.DataRev}
	for _, split := range SnapshotSplits {
		sf := files[split]
		if sf == nil {
			continue
		}
		if sf.rows != want[split] {
			return Snapshot{}, ErrDataChanged
		}
		if err := sf.bw.Flush(); err != nil {
			return Snapshot{}, fmt.Errorf("snapshot: flush %s: %w", sf.name, err)
		}
		if err := sf.f.Sync(); err != nil {
			return Snapshot{}, fmt.Errorf("snapshot: sync %s: %w", sf.name, err)
		}
		err := sf.f.Close()
		sf.f = nil
		if err != nil {
			return Snapshot{}, fmt.Errorf("snapshot: close %s: %w", sf.name, err)
		}
		if err := root.Rename(sf.name+".part", sf.name); err != nil {
			return Snapshot{}, fmt.Errorf("snapshot: publish %s: %w", sf.name, err)
		}
		snap.Files[split] = SnapshotFile{Name: sf.name, SHA256: hex.EncodeToString(sf.h.Sum(nil)), Rows: sf.rows, Bytes: sf.n}
		snap.Bytes += sf.n
	}
	marker, err := marshalStable(snapshotMarker{V: 1, RunID: runID, Format: format, DataRev: proj.DataRev, Files: snap.Files})
	if err != nil {
		return Snapshot{}, fmt.Errorf("snapshot: encode marker: %w", err)
	}
	mname := MarkerFileName(runID)
	if err := writeRootFile(root, mname, marker); err != nil {
		return Snapshot{}, fmt.Errorf("snapshot: write marker: %w", err)
	}
	sum := sha256.Sum256(marker)
	snap.Marker = SnapshotFile{Name: mname, SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(marker))}
	snap.Bytes += snap.Marker.Bytes
	snap.DataSHA = snapshotDataSHA(snap.Files)
	return snap, nil
}

func snapshotDataSHA(files map[string]SnapshotFile) string {
	h := sha256.New()
	for _, split := range SnapshotSplits {
		if f, ok := files[split]; ok {
			fmt.Fprintf(h, "%s:%s\n", split, f.SHA256)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func removeQuiet(root *os.Root, name string) error {
	if err := root.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// writeRootFile publishes data under root through a synced .part file.
func writeRootFile(root *os.Root, name string, data []byte) error {
	f, err := root.OpenFile(name+".part", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return root.Rename(name+".part", name)
}
