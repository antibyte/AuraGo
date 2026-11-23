package chibisensei

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"aurago/internal/fileutil"
)

// jobFacts are the job.json fields the later states need (read from runs.job_json, never from the catalog).
type jobFacts struct {
	Recipe   string   `json:"recipe"`
	TaskKind *string  `json:"task_kind"`
	Labels   []string `json:"labels"`
	Profile  struct {
		Base struct {
			Revision string `json:"revision"`
		} `json:"base"`
	} `json:"profile"`
	Deps struct {
		LlamaCpp struct {
			Tag string `json:"tag"`
		} `json:"llama_cpp"`
	} `json:"deps"`
}

func parseJobFacts(b []byte) (jobFacts, error) {
	var f jobFacts
	if err := json.Unmarshal(b, &f); err != nil {
		return jobFacts{}, fmt.Errorf("job facts: %w", err)
	}
	return f, nil
}

func (f jobFacts) converter() string { return BaseGGUFConverter(f.Deps.LlamaCpp.Tag) }

func (f jobFacts) taskKind() string {
	if f.TaskKind == nil {
		return ""
	}
	return *f.TaskKind
}

type exhaustedFunc func(ctx context.Context, r Run, t kaggleTurn) (stepResult, error)

// downloading: a final log read; chibi-status.json; chibi-manifest.json; the manifest's files (one call per step);
// then metrics, report and the next state.
func (o *Orchestrator) stepDownloading(ctx context.Context, r Run) (stepResult, error) {
	rem := r.Remote
	if !rem.FinalLogRead {
		return o.finalLog(ctx, r)
	}
	facts, err := parseJobFacts(r.JobJSON)
	if err != nil {
		return o.fail(ctx, r, "packaging_failed", "The job of the run is unreadable.", nil)
	}
	st := findOutput(rem.Outputs, OutStatus)
	if st == nil || !st.Copied {
		return o.fetch(ctx, r, facts, RunOutput{Path: OutStatus, Kind: "status", Bytes: -1},
			func(ctx context.Context, r Run, t kaggleTurn) (stepResult, error) {
				return o.fail(ctx, r, "status_missing", msgStatusMissing, t.save)
			})
	}
	b, err := o.readOutput(r, facts, "status", smallOutputMax)
	var status Status
	if err == nil {
		status, err = ParseStatus(b)
	}
	if err != nil || status.RunID != r.ID {
		return o.fail(ctx, r, "status_missing", msgStatusMissing, nil)
	}
	mf := findOutput(rem.Outputs, OutManifest)
	if mf == nil || (!mf.Copied && !mf.Missing) {
		return o.fetch(ctx, r, facts, RunOutput{Path: OutManifest, Kind: "manifest", Bytes: -1},
			func(ctx context.Context, r Run, t kaggleTurn) (stepResult, error) {
				if status.Succeeded() {
					return o.fail(ctx, r, "download_failed", msgDownloadFailed, t.save)
				}
				_, err := o.commit(ctx, r, NewRunEvent("warn", map[string]any{"code": "manifest_missing"}), func(x *Run) {
					t.save(x)
					putOutput(x, RunOutput{Path: OutManifest, Kind: "manifest", Bytes: -1, Missing: true})
				})
				return stepResult{progressed: true}, err
			})
	}
	if !rem.OutputsPlanned {
		return o.planOutputs(ctx, r, facts, status, *st, *mf)
	}
	for _, out := range rem.Outputs {
		if out.Copied || out.Missing || out.Kind == "status" || out.Kind == "manifest" {
			continue
		}
		return o.fetch(ctx, r, facts, out, func(ctx context.Context, r Run, t kaggleTurn) (stepResult, error) {
			return o.fail(ctx, r, "download_failed", msgDownloadFailed, t.save)
		})
	}
	return o.finishDownloads(ctx, r, facts, status)
}

func (o *Orchestrator) finalLog(ctx context.Context, r Run) (stepResult, error) {
	l, err := o.readLog(ctx, r)
	if err != nil {
		return stepResult{}, err
	}
	if err := o.st.UpsertMetrics(ctx, r.ID, l.metrics); err != nil {
		return o.localError(ctx, r, err)
	}
	next, err := o.commit(ctx, r, RunEvent{}, func(x *Run) { l.apply(x); x.Remote.FinalLogRead = true })
	if err != nil {
		return o.localError(ctx, r, err)
	}
	o.afterLog(ctx, next, l)
	return stepResult{progressed: true}, nil
}

// fetch moves one output one call further. A new attempt first checks the limits, drops a stale staged copy and
// stores its request ID; then download_kernel_output into r<run12>-<name>; on completion the copy is verified into
// Chibi storage, registered and the staged copy removed. failed, unknown or a vanished staged copy → attempt+1
// (at most 3, then exhausted).
func (o *Orchestrator) fetch(ctx context.Context, r Run, facts jobFacts, out RunOutput, exhausted exhaustedFunc) (stepResult, error) {
	rem := r.Remote
	step := StepDownloadPrefix + out.Path
	attempt := rem.Attempts[step]
	id := KaggleRequestID(r.ID, step, attempt)
	staged := stagedOutputName(r.ID, out.Path)
	if rec := rem.Step(step); rec == nil || rec.RequestID != id {
		if code, msg := o.checkRoom(out); code != "" {
			return o.fail(ctx, r, code, msg, nil)
		}
		if err := o.drv.RemoveFile(rem.KaggleProject, staged); err != nil {
			return o.blocked(ctx, r, err)
		}
		prep := KaggleStep{Step: step, Attempt: attempt, RequestID: id, File: out.Path, Updated: ts(o.now())}
		_, err := o.commit(ctx, r, RunEvent{}, func(x *Run) { x.Remote.PutStep(prep); putOutput(x, out) })
		return stepResult{progressed: true}, err
	}
	params := map[string]any{"ownerSlug": rem.Owner, "kernelSlug": rem.NotebookSlug, "filePath": out.Path}
	if rem.KernelVersion > 0 {
		params["versionNumber"] = rem.KernelVersion
	}
	t, err := o.kaggleStep(ctx, r, step, "download_kernel_output", rem.KaggleProject, staged, params)
	if err != nil {
		return o.localError(ctx, r, err)
	}
	if res, wait, err := o.pending(ctx, r, t); wait {
		return res, err
	}
	if t.job.Status != KaggleJobCompleted {
		return o.nextOutputAttempt(ctx, r, t, step, exhausted)
	}
	rel, err := outputRel(r, facts, out.Kind)
	if err != nil {
		return o.fail(ctx, r, "manifest_invalid", msgManifestInvalid, t.save)
	}
	src, err := o.drv.OpenFile(rem.KaggleProject, staged)
	if errors.Is(err, fs.ErrNotExist) {
		return o.nextOutputAttempt(ctx, r, t, step, exhausted)
	}
	if err != nil {
		return o.blocked(ctx, r, err)
	}
	limit := out.Bytes
	if limit < 0 {
		limit = smallOutputMax
	}
	root, err := openChibiRoot(o.dataDir)
	if err != nil {
		src.Close()
		return o.localError(ctx, r, err)
	}
	n, sum, err := copyVerified(root, rel, src, out.Bytes, out.SHA256, limit)
	src.Close()
	root.Close()
	switch {
	case errors.Is(err, errChecksumMismatch):
		return o.fail(ctx, r, "checksum_mismatch", msgChecksum, t.save)
	case errors.Is(err, errOutputTooLarge):
		return o.fail(ctx, r, "artifact_too_large", "A result file is larger than its manifest entry.", t.save)
	case err != nil:
		return o.localError(ctx, r, err)
	}
	got := out
	got.Bytes, got.SHA256, got.Copied = n, sum, true
	if err := o.register(ctx, r, facts, got, rel); err != nil {
		return o.localError(ctx, r, err)
	}
	if err := o.drv.RemoveFile(rem.KaggleProject, staged); err != nil {
		o.log.Warn("Chibi Sensei staged download not removed", "run", r.ID, "file", staged, "error", err)
	}
	_, err = o.commit(ctx, r, NewRunEvent("kaggle", map[string]any{"step": step, "status": KaggleJobCompleted, "bytes": n}),
		func(x *Run) { t.save(x); putOutput(x, got) })
	return stepResult{progressed: true}, err
}

func (o *Orchestrator) nextOutputAttempt(ctx context.Context, r Run, t kaggleTurn, step string, exhausted exhaustedFunc) (stepResult, error) {
	n := r.Remote.Attempts[step] + 1
	if n >= maxStepAttempts {
		return exhausted(ctx, r, t)
	}
	_, err := o.commit(ctx, r, NewRunEvent("kaggle", map[string]any{"step": step, "attempt": n, "status": t.job.Status}),
		func(x *Run) { t.save(x); setAttempt(x, step, n) })
	return stepResult{progressed: true}, err
}

// checkRoom refuses a download that exceeds kaggle.max_transfer_mb, the storage cap or free disk, before anything
// is transferred.
func (o *Orchestrator) checkRoom(out RunOutput) (string, string) {
	cfg := o.cfg()
	need := out.Bytes
	if need < 0 {
		need = smallOutputMax
	}
	if cfg.MaxTransferBytes > 0 && need > cfg.MaxTransferBytes {
		return "artifact_too_large", "A result file is larger than kaggle.max_transfer_mb allows."
	}
	if cfg.StorageMaxBytes > 0 && StorageBytes(o.dataDir)+o.st.SizeBytes()+need > cfg.StorageMaxBytes {
		return "storage_full", "The Chibi Sensei storage limit cannot hold the results."
	}
	if free, err := fileutil.FreeDiskBytes(o.dataDir); err == nil && free < need+minFreeDiskBytes {
		return "storage_full", "The disk has no room for the results."
	}
	return "", ""
}

// planOutputs puts the manifest's files into Outputs: all of them for a succeeded status (gguf_tuned required, the
// status entry must match the downloaded status), only metrics for a failed one.
func (o *Orchestrator) planOutputs(ctx context.Context, r Run, facts jobFacts, status Status, st, manifest RunOutput) (stepResult, error) {
	ok := status.Succeeded()
	var list []RunOutput
	if manifest.Copied {
		b, err := o.readOutput(r, facts, "manifest", smallOutputMax)
		var m Manifest
		if err == nil {
			m, err = ParseManifest(b)
		}
		if err == nil && (m.RunID != r.ID || m.ProfileID != r.ProfileID || m.ProfileSHA != r.ProfileSHA) {
			err = errors.New("the manifest belongs to another run")
		}
		if err != nil {
			if ok {
				return o.fail(ctx, r, "manifest_invalid", msgManifestInvalid, nil)
			}
			m = Manifest{}
		}
		for _, f := range m.Files {
			if f.Kind == "status" {
				if ok && (f.Bytes != st.Bytes || f.SHA256 != st.SHA256) {
					return o.fail(ctx, r, "checksum_mismatch", msgChecksum, nil)
				}
				continue
			}
			if !ok && f.Kind != "metrics" {
				continue
			}
			out := RunOutput{Path: f.Path, Kind: f.Kind, Bytes: f.Bytes, SHA256: f.SHA256}
			if f.Kind == "gguf_base" {
				reused, err := o.reuseBaseGGUF(ctx, r, facts, out)
				if err != nil {
					return o.localError(ctx, r, err)
				}
				out.Copied = reused
			}
			list = append(list, out)
		}
	}
	if ok && findKind(list, "gguf_tuned") == nil {
		return o.fail(ctx, r, "manifest_invalid", msgManifestInvalid, nil)
	}
	_, err := o.commit(ctx, r, NewRunEvent("kaggle", map[string]any{"step": "outputs", "files": len(list)}), func(x *Run) {
		for _, out := range list {
			putOutput(x, out)
		}
		x.Remote.OutputsPlanned = true
	})
	return stepResult{progressed: true}, err
}

// reuseBaseGGUF registers an identical base GGUF that is already stored instead of downloading it again.
func (o *Orchestrator) reuseBaseGGUF(ctx context.Context, r Run, facts jobFacts, out RunOutput) (bool, error) {
	b, found, err := o.st.GetBaseGGUF(ctx, r.ProfileID, facts.Profile.Base.Revision, facts.converter())
	if err != nil || !found || b.SHA256 != out.SHA256 {
		return false, err
	}
	root, err := openChibiRoot(o.dataDir)
	if err != nil {
		return false, err
	}
	defer root.Close()
	if info, err := root.Stat(b.RelPath); err != nil || info.Size() != out.Bytes {
		return false, nil
	}
	return true, o.st.AddArtifact(ctx, Artifact{RunID: r.ID, ProjectID: r.ProjectID, Kind: "gguf_base", RelPath: b.RelPath,
		Bytes: out.Bytes, SHA256: out.SHA256})
}

func (o *Orchestrator) finishDownloads(ctx context.Context, r Run, facts jobFacts, status Status) (stepResult, error) {
	if out := findKind(r.Remote.Outputs, "metrics"); out != nil && out.Copied {
		f, err := o.openOutput(r, facts, "metrics")
		if err != nil {
			return o.localError(ctx, r, err)
		}
		points, err := ParseMetricsFile(f)
		f.Close()
		if err != nil {
			return o.localError(ctx, r, err)
		}
		if err := o.st.ReplaceMetrics(ctx, r.ID, points); err != nil {
			return o.localError(ctx, r, err)
		}
	}
	if out := findKind(r.Remote.Outputs, "eval_samples"); status.Succeeded() && out != nil && out.Copied {
		f, err := o.openOutput(r, facts, "eval_samples")
		if err != nil {
			return o.localError(ctx, r, err)
		}
		limit := 0
		if r.Remote.Plan != nil {
			limit = r.Remote.Plan.EvalSamples
		}
		samples, err := ParseEvalSamples(f, limit)
		f.Close()
		if err != nil {
			return o.localError(ctx, r, err)
		}
		rep := ScoreReport(facts.Recipe, facts.taskKind(), facts.Labels, status, samples)
		rep.RunID = r.ID
		if err := o.st.SaveReport(ctx, r.ID, "auto", rep); err != nil {
			return o.localError(ctx, r, err)
		}
	}
	res := runResult(status)
	warning := status.WarningCode
	if warning == "" && status.Train.StopReason == "time_budget" {
		warning = "time_budget"
	}
	mut := func(x *Run) { x.Remote.Result, x.Remote.WarningCode = &res, warning }
	if !status.Succeeded() {
		code := status.Code
		if code == "" {
			code = "trainer_failed"
		}
		return o.fail(ctx, r, code, scrubText(status.Message, 500), mut)
	}
	return o.transition(ctx, r, RunVerifying, map[string]any{"stop_reason": status.Train.StopReason}, mut)
}

func (o *Orchestrator) readOutput(r Run, facts jobFacts, kind string, limit int64) ([]byte, error) {
	rel, err := outputRel(r, facts, kind)
	if err != nil {
		return nil, err
	}
	return readChibiFile(o.dataDir, rel, limit)
}

func (o *Orchestrator) openOutput(r Run, facts jobFacts, kind string) (*os.File, error) {
	rel, err := outputRel(r, facts, kind)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(ChibiDir(o.dataDir))
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.Open(rel)
}

func (o *Orchestrator) register(ctx context.Context, r Run, facts jobFacts, out RunOutput, rel string) error {
	if out.Kind == "manifest" {
		return nil
	}
	if err := o.st.AddArtifact(ctx, Artifact{RunID: r.ID, ProjectID: r.ProjectID, Kind: out.Kind, RelPath: rel,
		Bytes: out.Bytes, SHA256: out.SHA256}); err != nil {
		return err
	}
	if out.Kind != "gguf_base" {
		return nil
	}
	return o.st.PutBaseGGUF(ctx, BaseGGUF{ProfileID: r.ProfileID, BaseRevision: facts.Profile.Base.Revision,
		Converter: facts.converter(), RelPath: rel, SHA256: out.SHA256})
}

func findOutput(outs []RunOutput, p string) *RunOutput {
	for i := range outs {
		if outs[i].Path == p {
			return &outs[i]
		}
	}
	return nil
}

func findKind(outs []RunOutput, kind string) *RunOutput {
	for i := range outs {
		if outs[i].Kind == kind {
			return &outs[i]
		}
	}
	return nil
}

func putOutput(x *Run, out RunOutput) {
	if cur := findOutput(x.Remote.Outputs, out.Path); cur != nil {
		*cur = out
		return
	}
	x.Remote.Outputs = append(x.Remote.Outputs, out)
}

func runResult(s Status) RunResult {
	return RunResult{Steps: s.Train.Steps, Epochs: s.Train.Epochs, TrainLoss: finitePtr(s.Train.TrainLoss),
		EvalLossBase: finitePtr(s.Train.EvalLossBase), EvalLossTuned: finitePtr(s.Train.EvalLossTuned),
		StopReason: s.Train.StopReason, GPU: s.GPU.Name, Versions: &RunVersions{Python: s.Versions.Python,
			Torch: s.Versions.Torch, Transformers: s.Versions.Transformers, PEFT: s.Versions.PEFT, LlamaCpp: s.Versions.LlamaCpp}}
}

// verifying: GGUF header of the tuned model (and of the base model when present) against profile.gguf.expected_arch.
// A bad base GGUF is dropped with the warning base_gguf_invalid; a bad tuned model fails the run.
func (o *Orchestrator) stepVerifying(ctx context.Context, r Run) (stepResult, error) {
	prof, ok := o.cat.Get(r.ProfileID)
	facts, err := parseJobFacts(r.JobJSON)
	if !ok || err != nil {
		return o.fail(ctx, r, "profile_changed", "The model profile is no longer available.", nil)
	}
	root, err := openChibiRoot(o.dataDir)
	if err != nil {
		return o.localError(ctx, r, err)
	}
	defer root.Close()
	rel, _ := outputRel(r, facts, "gguf_tuned")
	info, err := readGGUFInRoot(root, rel)
	if err != nil {
		if !errors.Is(err, ErrGGUFInvalid) && !errors.Is(err, fs.ErrNotExist) {
			return o.localError(ctx, r, err)
		}
		return o.fail(ctx, r, "gguf_failed", "The trained model is not a valid GGUF file.", nil)
	}
	if info.Architecture != prof.GGUF.ExpectedArch {
		return o.fail(ctx, r, "gguf_arch_mismatch", fmt.Sprintf("The model file has architecture %q instead of %q.",
			info.Architecture, prof.GGUF.ExpectedArch), nil)
	}
	warning := r.Remote.WarningCode
	if base := findKind(r.Remote.Outputs, "gguf_base"); base != nil && base.Copied {
		brel, err := outputRel(r, facts, "gguf_base")
		if err != nil {
			return o.localError(ctx, r, err)
		}
		if binfo, err := readGGUFInRoot(root, brel); err != nil || binfo.Architecture != prof.GGUF.ExpectedArch {
			if err := o.dropBaseGGUF(ctx, r, facts, root, brel); err != nil {
				return o.localError(ctx, r, err)
			}
			if warning == "" {
				warning = "base_gguf_invalid"
			}
		}
	}
	return o.transition(ctx, r, RunCleaning, map[string]any{"end_state": RunSucceeded, "gguf": info}, func(x *Run) {
		x.Remote.EndState, x.Remote.GGUF, x.Remote.WarningCode = RunSucceeded, &info, warning
	})
}

func (o *Orchestrator) dropBaseGGUF(ctx context.Context, r Run, facts jobFacts, root *os.Root, rel string) error {
	if err := removeQuiet(root, rel); err != nil {
		return err
	}
	if err := o.st.deleteBaseGGUF(ctx, r.ProfileID, facts.Profile.Base.Revision, facts.converter()); err != nil {
		return err
	}
	return o.st.deleteArtifact(ctx, ArtifactID(r.ID, "gguf_base"))
}
