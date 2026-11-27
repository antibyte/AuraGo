package chibisensei

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ErrRunNotSucceeded means the operation needs a succeeded run.
var ErrRunNotSucceeded = errors.New("chibisensei: the training run did not succeed")

// ModelFile is one GGUF of a succeeded run as the Test step sees it.
type ModelFile struct {
	Available bool   `json:"available"`
	Bytes     int64  `json:"bytes"`
	Reason    string `json:"reason"` // "" | missing | skipped | invalid
	Key       string `json:"-"`      // chatrt.ModelRef.Key
	Path      string `json:"-"`      // absolute path, set only when Available
}

// RunTask returns the recipe, task kind and labels a run was trained for, read from its job.json.
func RunTask(run Run) (recipe, taskKind string, labels []string, err error) {
	f, err := parseJobFacts(run.JobJSON)
	if err != nil {
		return "", "", nil, err
	}
	return f.Recipe, f.taskKind(), f.Labels, nil
}

// RunModels resolves the tuned and the base GGUF of a succeeded run. Only registered artifact rows (and the
// shared base row) name files; each file is opened through os.Root, must be a regular file of the recorded size
// and must have the expected architecture header.
func RunModels(ctx context.Context, st *Store, dataDir string, run Run) (tuned, base ModelFile, err error) {
	if run.State != RunSucceeded {
		return ModelFile{}, ModelFile{}, ErrRunNotSucceeded
	}
	arts, err := st.ListArtifacts(ctx, run.ID)
	if err != nil {
		return ModelFile{}, ModelFile{}, fmt.Errorf("run models: %w", err)
	}
	var tunedArt, baseArt *Artifact
	for i := range arts {
		switch arts[i].Kind {
		case "gguf_tuned":
			tunedArt = &arts[i]
		case "gguf_base":
			baseArt = &arts[i]
		}
	}
	wantArch := ""
	if run.Remote.GGUF != nil {
		wantArch = run.Remote.GGUF.Architecture
	}
	var root *os.Root
	if r, rerr := os.OpenRoot(ChibiDir(dataDir)); rerr == nil {
		root = r
		defer root.Close()
	} else if !errors.Is(rerr, fs.ErrNotExist) {
		return ModelFile{}, ModelFile{}, fmt.Errorf("open %s: %w", chibiDirName, rerr)
	}
	tuned = ModelFile{Reason: "missing"}
	if tunedArt != nil {
		tuned = checkModel(root, dataDir, tunedArt.RelPath, tunedArt.Bytes, wantArch, "tuned:"+run.ID)
	}
	base = ModelFile{Reason: "skipped"}
	if baseArt != nil {
		base = checkModel(root, dataDir, baseArt.RelPath, baseArt.Bytes, wantArch, "base:"+baseArt.RelPath)
	} else if facts, ferr := parseJobFacts(run.JobJSON); ferr == nil {
		row, ok, gerr := st.GetBaseGGUF(ctx, run.ProfileID, facts.Profile.Base.Revision, facts.converter())
		if gerr != nil {
			return ModelFile{}, ModelFile{}, fmt.Errorf("run models: %w", gerr)
		}
		if ok {
			base = checkModel(root, dataDir, row.RelPath, -1, wantArch, "base:"+row.RelPath)
		}
	}
	return tuned, base, nil
}

// checkModel applies the D6 rules to one registered relative path; wantBytes < 0 accepts any size and an empty
// wantArch accepts any architecture.
func checkModel(root *os.Root, dataDir, rel string, wantBytes int64, wantArch, key string) ModelFile {
	if !validArtifactPath(rel) {
		return ModelFile{Reason: "invalid"}
	}
	if root == nil {
		return ModelFile{Reason: "missing"}
	}
	fi, err := root.Lstat(rel)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ModelFile{Reason: "missing"}
		}
		return ModelFile{Reason: "invalid"}
	}
	if !fi.Mode().IsRegular() {
		return ModelFile{Reason: "invalid"}
	}
	if wantBytes >= 0 && fi.Size() != wantBytes {
		return ModelFile{Reason: "missing"}
	}
	info, err := readGGUFInRoot(root, rel)
	if err != nil || (wantArch != "" && info.Architecture != wantArch) {
		return ModelFile{Reason: "invalid"}
	}
	return ModelFile{Available: true, Bytes: fi.Size(), Key: key,
		Path: filepath.Join(ChibiDir(dataDir), filepath.FromSlash(rel))}
}

// MarkReportSeen records that the report card of a succeeded run was opened. It is idempotent and writes no
// run event.
func (s *Store) MarkReportSeen(ctx context.Context, runID string) error {
	r, err := s.GetRun(ctx, runID)
	if err != nil {
		return err
	}
	if r.State != RunSucceeded {
		return ErrRunNotSucceeded
	}
	if r.Remote.ReportSeen {
		return nil
	}
	_, err = s.UpdateRun(ctx, runID, RunEvent{}, func(x *Run) error {
		if x.State != RunSucceeded {
			return ErrRunNotSucceeded
		}
		x.Remote.ReportSeen = true
		return nil
	})
	return err
}
