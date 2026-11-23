package chibisensei

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"aurago/internal/chibisensei/trainer"
)

var errSnapshotChanged = errors.New("chibisensei: the frozen snapshot changed")

func stagedFiles(s Snapshot) []StagedFile {
	var out []StagedFile
	for _, split := range SnapshotSplits {
		if f, ok := s.Files[split]; ok {
			out = append(out, StagedFile{Split: split, Name: f.Name, SHA256: f.SHA256, Bytes: f.Bytes, Rows: f.Rows})
		}
	}
	return append(out, StagedFile{Split: "marker", Name: s.Marker.Name, SHA256: s.Marker.SHA256, Bytes: s.Marker.Bytes})
}

// queued: freeze the snapshot, build job.json and the notebook (redone after a restart; local and idempotent).
func (o *Orchestrator) stepQueued(ctx context.Context, r Run) (stepResult, error) {
	plan := r.Remote.Plan
	if plan == nil {
		return o.fail(ctx, r, "packaging_failed", "The run has no stored plan.", nil)
	}
	prof, ok := o.cat.Get(r.ProfileID)
	if !ok || o.cat.SHA(r.ProfileID) != r.ProfileSHA {
		return o.fail(ctx, r, "profile_changed", "The model profile changed after the start.", nil)
	}
	proj, err := o.st.GetProject(ctx, r.ProjectID)
	if err != nil {
		return o.localError(ctx, r, fmt.Errorf("load project: %w", err))
	}
	if proj.DataRev != plan.DataRev {
		return o.fail(ctx, r, "data_changed", msgDataChanged, nil)
	}
	snap, err := o.st.FreezeSnapshot(ctx, r.ProjectID, r.ID, snapshotDir(o.dataDir, r.ID))
	var invalid *ValidationError
	switch {
	case errors.Is(err, ErrDataChanged) || errors.As(err, &invalid):
		return o.fail(ctx, r, "data_changed", msgDataChanged, nil)
	case err != nil:
		return o.localError(ctx, r, fmt.Errorf("freeze snapshot: %w", err))
	}
	if snap.DataRev != plan.DataRev || snap.Files["train"].Rows != plan.Split.Train ||
		snap.Files["val"].Rows != plan.Split.Val || snap.Files["eval"].Rows != plan.Split.Eval {
		return o.fail(ctx, r, "data_changed", msgDataChanged, nil)
	}
	created, err := time.Parse(time.RFC3339, r.Created)
	if err != nil {
		created = o.now()
	}
	lock, err := trainer.LoadLock()
	if err != nil {
		return o.fail(ctx, r, "packaging_failed", "The trainer lock file is invalid.", nil)
	}
	spec, err := BuildJobSpec(JobInput{RunID: r.ID, Created: created, AuraGoVersion: o.cfg().AuraGoVersion, Project: proj,
		Profile: prof, ProfileSHA: r.ProfileSHA, Plan: *plan, Marker: snap.Marker.Name, Format: snap.Format, Lock: lock})
	if err != nil {
		o.log.Warn("Chibi Sensei job spec refused", "run", r.ID, "error", err)
		return o.fail(ctx, r, "packaging_failed", "The training job could not be built.", nil)
	}
	job, err := spec.JSON()
	if err != nil {
		return o.localError(ctx, r, err)
	}
	nb, err := BuildNotebook(r.ID, job, trainer.Script())
	if err != nil {
		o.log.Warn("Chibi Sensei notebook refused", "run", r.ID, "error", err)
		return o.fail(ctx, r, "packaging_failed", "The notebook could not be built.", nil)
	}
	if err := writeChibiFile(o.dataDir, runRel(r.ID, notebookFileName), nb); err != nil {
		return o.localError(ctx, r, fmt.Errorf("write notebook: %w", err))
	}
	staged := stagedFiles(snap)
	return o.transition(ctx, r, RunPackaging, map[string]any{"files": len(staged), "bytes": snap.Bytes}, func(x *Run) {
		x.JobJSON, x.DataSHA = job, snap.DataSHA
		x.Remote.Staged, x.Remote.SnapshotBytes, x.Remote.Marker = staged, snap.Bytes, snap.Marker.Name
	})
}

// packaging: the Kaggle project first (stored so cleanup can find staged files), then every staged file whose
// project copy is missing or differs.
func (o *Orchestrator) stepPackaging(ctx context.Context, r Run) (stepResult, error) {
	project, err := o.drv.EnsureProject(ctx)
	if err != nil {
		return o.blocked(ctx, r, err)
	}
	if r.Remote.KaggleProject != project {
		_, err := o.commit(ctx, r, NewRunEvent("kaggle", map[string]any{"step": "project"}),
			func(x *Run) { x.Remote.KaggleProject = project })
		return stepResult{progressed: true}, err
	}
	root, err := os.OpenRoot(RunDir(o.dataDir, r.ID))
	if err != nil {
		return o.localError(ctx, r, err)
	}
	defer root.Close()
	for _, f := range r.Remote.Staged {
		if o.projectFileMatches(project, f) {
			continue
		}
		if err := o.stageFile(ctx, root, project, f); err != nil {
			if errors.Is(err, errSnapshotChanged) {
				return o.fail(ctx, r, "data_changed", msgDataChanged, nil)
			}
			return o.blocked(ctx, r, err)
		}
	}
	return o.transition(ctx, r, RunUploading, map[string]any{"files": len(r.Remote.Staged)}, nil)
}

func (o *Orchestrator) projectFileMatches(project string, f StagedFile) bool {
	src, err := o.drv.OpenFile(project, f.Name)
	if err != nil {
		return false
	}
	defer src.Close()
	h := sha256.New()
	n, err := io.Copy(h, src)
	return err == nil && n == f.Bytes && hex.EncodeToString(h.Sum(nil)) == f.SHA256
}

func (o *Orchestrator) stageFile(ctx context.Context, root *os.Root, project string, f StagedFile) error {
	src, err := root.Open(path.Join(snapshotSubdir, f.Name))
	if err != nil {
		return fmt.Errorf("%w: %v", errSnapshotChanged, err)
	}
	defer src.Close()
	h := sha256.New()
	n, err := o.drv.WriteFile(ctx, project, f.Name, io.TeeReader(src, h))
	if err != nil {
		return err
	}
	if n != f.Bytes || hex.EncodeToString(h.Sum(nil)) != f.SHA256 {
		_ = o.drv.RemoveFile(project, f.Name)
		return errSnapshotChanged
	}
	return nil
}

// uploading: one upload_dataset_file per staged file, in order. completed → next file; unknown → new attempt and new
// ID (at most 3); failed → the run fails.
func (o *Orchestrator) stepUploading(ctx context.Context, r Run) (stepResult, error) {
	files := r.Remote.Staged
	for i, f := range files {
		step := StepUploadPrefix + f.Split
		if rec := r.Remote.Step(step); rec != nil && rec.Status == KaggleJobCompleted && rec.UploadRef != "" {
			continue
		}
		t, err := o.kaggleStep(ctx, r, step, "upload_dataset_file", r.Remote.KaggleProject, f.Name, nil)
		if err != nil {
			return o.localError(ctx, r, err)
		}
		if res, wait, err := o.pending(ctx, r, t); wait {
			return res, err
		}
		switch t.job.Status {
		case KaggleJobCompleted:
			ref := jobFields(t.job).UploadRef
			if !strings.HasPrefix(ref, "upload:") {
				return o.fail(ctx, r, "upload_failed", msgUploadFailed, t.save)
			}
			if t.rec == nil {
				cur := *r.Remote.Step(step)
				t.rec = &cur
			}
			t.rec.UploadRef = ref
			if i == len(files)-1 {
				return o.transition(ctx, r, RunCreatingDataset, map[string]any{"uploads": len(files)}, t.save)
			}
			_, err := o.commit(ctx, r, kaggleEvent(*t.rec), t.save)
			return stepResult{progressed: true}, err
		case KaggleJobUnknown:
			return o.nextAttempt(ctx, r, t, step, "upload_failed", msgUploadFailed)
		default:
			return o.fail(ctx, r, "upload_failed", msgUploadFailed, t.save)
		}
	}
	return o.transition(ctx, r, RunCreatingDataset, nil, nil)
}

// creating_dataset: create_dataset with the upload refs. unknown → a get_dataset read decides: exists → done;
// 404 → new attempt.
func (o *Orchestrator) stepCreatingDataset(ctx context.Context, r Run) (stepResult, error) {
	rem := r.Remote
	files := make([]map[string]any, 0, len(rem.Staged))
	for _, f := range rem.Staged {
		rec := rem.Step(StepUploadPrefix + f.Split)
		if rec == nil || rec.UploadRef == "" {
			return o.fail(ctx, r, "upload_failed", msgUploadFailed, nil)
		}
		files = append(files, map[string]any{"token": rec.UploadRef})
	}
	params := map[string]any{"ownerSlug": rem.Owner, "slug": rem.DatasetSlug, "title": rem.DatasetSlug,
		"licenseName": "unknown", "isPrivate": true, "files": files}
	t, err := o.kaggleStep(ctx, r, StepCreateDataset, "create_dataset", "", "", params)
	if err != nil {
		return o.localError(ctx, r, err)
	}
	if res, wait, err := o.pending(ctx, r, t); wait {
		return res, err
	}
	switch t.job.Status {
	case KaggleJobCompleted:
		return o.datasetCreated(ctx, r, t)
	case KaggleJobUnknown:
		_, err := o.readOp(ctx, "get_dataset", map[string]any{"ownerSlug": rem.Owner, "datasetSlug": rem.DatasetSlug})
		switch {
		case err == nil:
			return o.datasetCreated(ctx, r, t)
		case IsKaggleNotFound(err):
			return o.nextAttempt(ctx, r, t, StepCreateDataset, "dataset_failed", msgDatasetFailed)
		}
		return o.blocked(ctx, r, err)
	}
	return o.fail(ctx, r, "dataset_failed", msgDatasetFailed, t.save)
}

func (o *Orchestrator) datasetCreated(ctx context.Context, r Run, t kaggleTurn) (stepResult, error) {
	removed := map[string]bool{}
	for _, f := range r.Remote.Staged {
		if err := o.drv.RemoveFile(r.Remote.KaggleProject, f.Name); err != nil {
			o.log.Warn("Chibi Sensei staged file not removed", "run", r.ID, "file", f.Name, "error", err)
			continue
		}
		removed[f.Name] = true
	}
	now := ts(o.now())
	return o.transition(ctx, r, RunWaitingDataset, map[string]any{"dataset": KaggleRef(r.Remote.Owner, r.Remote.DatasetSlug)},
		func(x *Run) {
			t.save(x)
			for i := range x.Remote.Staged {
				if removed[x.Remote.Staged[i].Name] {
					x.Remote.Staged[i].Removed = true
				}
			}
			x.Remote.DatasetWaitStarted = now
		})
}

// waiting_dataset: get_dataset_status every 15 s, up to 15 minutes of successful reads.
func (o *Orchestrator) stepWaitingDataset(ctx context.Context, r Run) (stepResult, error) {
	rem := r.Remote
	data, err := o.readOp(ctx, "get_dataset_status", map[string]any{"ownerSlug": rem.Owner, "datasetSlug": rem.DatasetSlug})
	if err != nil && !IsKaggleNotFound(err) {
		return o.blocked(ctx, r, err)
	}
	if err == nil {
		ready, failed := datasetReady(data)
		if ready {
			return o.transition(ctx, r, RunStarting, nil, nil)
		}
		if failed {
			return o.fail(ctx, r, "dataset_failed", msgDatasetFailed, nil)
		}
	}
	if rem.BlockedSince != "" || rem.BlockedCode != "" { // a refused wait restarts the 15-minute window
		now := ts(o.now())
		if _, err := o.commit(ctx, r, RunEvent{}, func(x *Run) { x.Remote.DatasetWaitStarted = now }); err != nil {
			return o.localError(ctx, r, err)
		}
		return stepResult{wait: o.iv.dataset}, nil
	}
	if since, perr := time.Parse(time.RFC3339, rem.DatasetWaitStarted); perr == nil && o.now().Sub(since) > datasetWaitMax {
		return o.fail(ctx, r, "dataset_timeout", "Kaggle did not finish the dataset within 15 minutes.", nil)
	}
	return stepResult{wait: o.iv.dataset}, nil
}

// starting: save_kernel SAVE_AND_RUN_ALL. remote_running → running. unknown → a get_kernel_session_status read:
// it answers → running untracked; 404 → kaggle_submit_unknown (nothing is sent again).
func (o *Orchestrator) stepStarting(ctx context.Context, r Run) (stepResult, error) {
	rem := r.Remote
	if cfg := o.cfg(); !submitted(r) && cfg.RemoteTimeoutMinutes > 0 && rem.SessionTimeoutMinutes > cfg.RemoteTimeoutMinutes {
		return o.fail(ctx, r, "submit_failed", "kaggle.remote_timeout_minutes is now shorter than the session of this run.", nil)
	}
	nb, err := readChibiFile(o.dataDir, runRel(r.ID, notebookFileName), maxNotebookBytes)
	if err != nil {
		return o.fail(ctx, r, "packaging_failed", "The notebook of the run is missing.", nil)
	}
	params := map[string]any{"slug": KaggleRef(rem.Owner, rem.NotebookSlug), "newTitle": rem.NotebookSlug,
		"text": string(nb), "kernelType": "notebook", "language": "python", "kernelExecutionType": "SAVE_AND_RUN_ALL",
		"isPrivate": true, "enableGpu": true, "enableInternet": true, "machineShape": rem.MachineShape,
		"datasetDataSources":    []string{KaggleRef(rem.Owner, rem.DatasetSlug)},
		"sessionTimeoutSeconds": rem.SessionTimeoutMinutes * 60}
	t, err := o.kaggleStep(ctx, r, StepSaveKernel, "save_kernel", "", "", params)
	if err != nil {
		return o.localError(ctx, r, err)
	}
	if t.rec != nil && t.rec.JobID != "" && rem.Submitted == "" {
		at := ts(o.now())
		t.extra = func(x *Run) { x.Remote.Submitted = at }
	}
	if res, wait, err := o.pending(ctx, r, t); wait {
		return res, err
	}
	f := jobFields(t.job)
	switch {
	case t.job.Status == KaggleJobUnknown:
		data, err := o.readOp(ctx, "get_kernel_session_status", map[string]any{"userName": rem.Owner, "kernelSlug": rem.NotebookSlug})
		switch {
		case err == nil:
			return o.startRunning(ctx, r, t, true, 1, sessionStatus(data))
		case IsKaggleNotFound(err) && r.CancelRequested:
			return o.endRun(ctx, r, RunCancelled, codeCancelled, "", t.save)
		case IsKaggleNotFound(err):
			return o.fail(ctx, r, "kaggle_submit_unknown", "Kaggle did not confirm the notebook start. Nothing was sent again.", t.save)
		}
		return o.blocked(ctx, r, err)
	case t.job.Status == KaggleJobRemoteRunning || t.job.Status == KaggleJobCancelRequested || f.VersionNumber > 0:
		return o.startRunning(ctx, r, t, false, f.VersionNumber, f.RemoteStatus)
	case r.CancelRequested:
		return o.endRun(ctx, r, RunCancelled, codeCancelled, "", t.save)
	case t.job.Error == "concurrent Kaggle compute limit reached":
		return o.fail(ctx, r, "kaggle_slot_busy", "Another Kaggle notebook run holds the compute slot.", t.save)
	}
	return o.fail(ctx, r, "submit_failed", "Kaggle refused the notebook start.", t.save)
}

func (o *Orchestrator) startRunning(ctx context.Context, r Run, t kaggleTurn, untracked bool, version int, status string) (stepResult, error) {
	now := o.now().UTC()
	if version < 1 {
		version = 1 // the slug is new per run, so its first and only version is 1
	}
	kernelID := jobFields(t.job).KernelID
	deadline := now.Add(time.Duration(r.Remote.SessionTimeoutMinutes)*time.Minute + deadlineSlack)
	return o.transition(ctx, r, RunRunning, map[string]any{"untracked": untracked, "deadline": ts(deadline)}, func(x *Run) {
		t.save(x)
		if x.Remote.Submitted == "" {
			x.Remote.Submitted = ts(now)
		}
		x.Remote.Untracked, x.Remote.KernelVersion, x.Remote.KernelID = untracked, version, kernelID
		x.Remote.RemoteStatus, x.Remote.RemoteStarted, x.Deadline = status, ts(now), ts(deadline)
	})
}
