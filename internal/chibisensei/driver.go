package chibisensei

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"aurago/internal/kaggle"
)

// KaggleProjectTitle is the title of the one local Kaggle project that stages Chibi files.
const KaggleProjectTitle = "Chibi Sensei"

// SettingKaggleProjectID is the Chibi settings key that keeps that project's id.
const SettingKaggleProjectID = "kaggle_project_id"

// Kaggle job statuses (string literals in internal/kaggle).
const (
	KaggleJobQueued          = "queued"
	KaggleJobRunning         = "running"
	KaggleJobUnknown         = "unknown"
	KaggleJobRemoteRunning   = "remote_running"
	KaggleJobCancelRequested = "cancel_requested"
	KaggleJobCompleted       = "completed"
	KaggleJobFailed          = "failed"
	KaggleJobCancelled       = "cancelled"
	KaggleJobResolvedUnknown = "resolved_unknown"
)

// KaggleJobActive reports whether a status holds (or may hold) a compute slot, as checkComputeSlots counts it.
func KaggleJobActive(status string) bool {
	switch status {
	case KaggleJobQueued, KaggleJobRunning, KaggleJobUnknown, KaggleJobRemoteRunning, KaggleJobCancelRequested:
		return true
	}
	return false
}

// KaggleJobTerminal reports whether a status is final (unknown is not: it needs an administrator).
func KaggleJobTerminal(status string) bool {
	switch status {
	case KaggleJobCompleted, KaggleJobFailed, KaggleJobCancelled, KaggleJobResolvedUnknown:
		return true
	}
	return false
}

// KaggleRequestID is the deterministic request ID of one step attempt: the first 32 hex characters of
// sha256(run_id + ":" + step + ":" + attempt).
func KaggleRequestID(runID, step string, attempt int) string {
	sum := sha256.Sum256([]byte(runID + ":" + step + ":" + strconv.Itoa(attempt)))
	return hex.EncodeToString(sum[:])[:32]
}

// IsKaggleNotFound reports a Kaggle 404 (for example a read of a dataset that does not exist).
func IsKaggleNotFound(err error) bool {
	var ae *kaggle.APIError
	return errors.As(err, &ae) && ae.Status == 404
}

// IsKaggleUncertain reports an error whose remote outcome is unknown.
func IsKaggleUncertain(err error) bool {
	var ae *kaggle.APIError
	return errors.As(err, &ae) && ae.Uncertain
}

var kaggleOpDomains = map[string]string{
	"upload_dataset_file": "datasets", "create_dataset": "datasets", "get_dataset": "datasets",
	"get_dataset_status": "datasets", "delete_dataset": "datasets",
	"save_kernel": "notebooks", "get_kernel": "notebooks", "get_kernel_session_status": "notebooks",
	"list_kernel_session_output": "notebooks", "download_kernel_output": "notebooks", "delete_kernel": "notebooks",
	"get_accelerator_quota_statistics": "notebooks",
}

// NewKaggleRequest builds a request for an operation Chibi uses. params is marshalled as the SDK wire object
// (field names of the pinned contract, e.g. "ownerSlug"); nil gives {}.
func NewKaggleRequest(id, op, project, file string, params any) (kaggle.Request, error) {
	domain, ok := kaggleOpDomains[op]
	if !ok {
		return kaggle.Request{}, fmt.Errorf("kaggle request: operation %q is not used by Chibi Sensei", op)
	}
	if params == nil {
		params = map[string]any{}
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return kaggle.Request{}, fmt.Errorf("kaggle request %s: %w", op, err)
	}
	return kaggle.Request{ID: id, Domain: domain, Operation: op, Parameters: raw, ProjectID: project, File: file}, nil
}

// KaggleDriver is the orchestrator's only path to Kaggle. B5 implements it over *kaggle.Service with the current
// kaggleSnapshot().Access; fakeKaggle implements it for tests.
type KaggleDriver interface {
	// Account returns the authenticated Kaggle account (Service.Test → "account"). It needs network access and never
	// mutates; an error means the account is unknown right now (credentials, network or Kaggle).
	Account(ctx context.Context) (string, error)

	// Execute runs one catalog operation.
	//
	// Reads (grant read: get_dataset, get_dataset_status, get_kernel, get_kernel_session_status,
	// list_kernel_session_output, get_accelerator_quota_statistics) run synchronously. Result.Data is the scrubbed
	// JSON answer ({} when empty), Result.Job is nil, req.ID is ignored and nothing is persisted. A missing resource
	// is an error for which IsKaggleNotFound is true.
	//
	// Every other operation (upload_dataset_file, create_dataset, save_kernel, download_kernel_output,
	// delete_dataset, delete_kernel) is a durable job. Execute admits it and returns at once with Result.Job in
	// status queued (or already failed when the compute slot check refused a SAVE_AND_RUN_ALL: Job.Error
	// "concurrent Kaggle compute limit reached", nil error). The job then runs in the Kaggle service; poll it with Job.
	//
	// req.ID must be 32 hex characters (KaggleRequestID); the job ID equals req.ID. Execute with a known ID and the
	// same canonical request (account, domain, operation, project, file, parameters) returns the stored job in its
	// current status and does nothing else. A known ID with a different request is an error. An error means the
	// request was refused before it ran (validation, grants, policy, account, timeout bound, the host's job gate).
	// Only the host gate admits the job as failed before it returns the error; so after any error the same ID
	// can be retried safely: it is either admitted fresh or returns the original receipt.
	//
	// Job statuses: queued → running → completed | failed | unknown. unknown means the outcome is uncertain (an
	// uncertain network error, or an AuraGo restart while queued or running); the service never retries it and a
	// replay of the ID returns unknown again, so a new attempt needs a new ID (attempt+1). save_kernel with
	// kernelExecutionType SAVE_AND_RUN_ALL ends running as remote_running; the service polls the session every
	// 10 s, writes Result.remote_status (upper-case Kaggle status) and ends completed | failed | cancelled.
	// remote_running survives a restart. Result of save_kernel: ref, url, versionNumber, kernelId; a
	// kernelSessionId is not promised.
	//
	// The service applies defaults: save_kernel isPrivate true, enableGpu/enableTpu/enableInternet false unless
	// given, sessionTimeoutSeconds must lie in (0, remote_timeout_minutes×60] (otherwise an error); create_dataset
	// isPrivate true and ownerSlug = account.
	//
	// upload_dataset_file needs ProjectID and File (the staged project file) and Parameters {}; the service fills
	// fileName (= File), contentLength and lastModifiedEpochSeconds. Completed Result:
	// {"upload_ref":"upload:<job id>","bytes":n,"file":name}; upload_ref goes into create_dataset files[].token.
	// download_kernel_output needs ProjectID and File, a project file name that must not exist yet (an existing
	// file fails the job with "project file revision changed"). Completed Result: {"project_id","file","bytes"}.
	// Job.Result and Job.Error are public, scrubbed text.
	Execute(ctx context.Context, req kaggle.Request) (kaggle.Result, error)

	// Job returns the stored job (Store.Get). Reading never changes the job on the real service. An unknown id is
	// an error (sql.ErrNoRows on the real store).
	Job(ctx context.Context, id string) (kaggle.Job, error)

	// ActiveComputeJobs lists the jobs that hold a compute slot: status queued, running, unknown, remote_running or
	// cancel_requested and operation save_kernel with SAVE_AND_RUN_ALL (or a benchmark batch). It includes jobs of
	// the agent and other apps; an unknown job stays until an administrator resolves it in the Kaggle app.
	ActiveComputeJobs(ctx context.Context) ([]kaggle.Job, error)

	// EnsureProject returns the id (32 hex) of the local Kaggle project titled KaggleProjectTitle. It creates the
	// project on first use and keeps its id under SettingKaggleProjectID; later calls return the same id while the
	// project exists and recreate it when it is gone. Needs Kaggle enabled and filesystem write access.
	EnsureProject(ctx context.Context) (string, error)

	// WriteFile writes r to project/name and replaces an existing regular file atomically, within max_transfer_mb,
	// the free part of max_storage_mb and free disk minus 64 MiB. Names follow the Kaggle package rules (no
	// separators, no leading dot, under 240 bytes). It returns the bytes written.
	WriteFile(ctx context.Context, project, name string, r io.Reader) (int64, error)

	// OpenFile opens a regular project file for reading; the caller closes it. A missing file matches os.ErrNotExist.
	OpenFile(project, name string) (*os.File, error)

	// RemoveFile deletes one regular project file; a missing file is not an error.
	RemoveFile(project, name string) error

	// ReadKernelLog reads the session log of one notebook version without a durable job and without a project file:
	// at most max bytes from the end (the read itself is capped at 16 MiB). An empty body with a nil error means
	// "no log yet". Needs the download grant, filesystem write and network; owner must be the authenticated account.
	// The body is raw (a JSON array of {stream_name,time,data} or plain text); the caller scrubs before storing.
	ReadKernelLog(ctx context.Context, owner, slug string, version int, max int64) ([]byte, error)

	// Cancel is Service.Cancel. queued or running: cancels the local job (it ends failed or unknown).
	// remote_running: sends cancel_kernel_session, which needs a kernelSessionId that SaveKernel normally does not
	// return, so it normally returns an error and the job stays remote_running (the user stops the notebook on
	// Kaggle). cancel_requested: nil. Any other status: error "job is not active".
	Cancel(ctx context.Context, jobID string) error
}
