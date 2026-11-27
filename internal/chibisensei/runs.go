package chibisensei

import (
	"encoding/json"
	"errors"

	"github.com/google/uuid"
)

// Run states in lifecycle order (spec "Run state machine").
const (
	RunQueued          = "queued"
	RunPackaging       = "packaging"
	RunUploading       = "uploading"
	RunCreatingDataset = "creating_dataset"
	RunWaitingDataset  = "waiting_dataset"
	RunStarting        = "starting"
	RunRunning         = "running"
	RunDownloading     = "downloading"
	RunVerifying       = "verifying"
	RunCleaning        = "cleaning"
	RunSucceeded       = "succeeded"
	RunFailed          = "failed"
	RunCancelled       = "cancelled"
)

var runStates = []string{RunQueued, RunPackaging, RunUploading, RunCreatingDataset, RunWaitingDataset, RunStarting,
	RunRunning, RunDownloading, RunVerifying, RunCleaning, RunSucceeded, RunFailed, RunCancelled}

// RunStates lists every run state in lifecycle order (GET /meta "run_states").
func RunStates() []string { return append([]string(nil), runStates...) }

// RunTerminal reports whether state is succeeded, failed or cancelled.
func RunTerminal(state string) bool {
	return state == RunSucceeded || state == RunFailed || state == RunCancelled
}

func validRunState(state string) bool {
	for _, s := range runStates {
		if s == state {
			return true
		}
	}
	return false
}

// terminalStatesSQL is the SQL list of the terminal states.
const terminalStatesSQL = `('succeeded','failed','cancelled')`

// BackendKaggle is the only training backend of P2.
const BackendKaggle = "kaggle"

// Values of RunRemote.Live and the API field "live".
const (
	LiveYes     = "yes"
	LiveNo      = "no"
	LiveUnknown = "unknown"
)

// Kaggle step names. A step name plus its attempt gives the request ID (KaggleRequestID).
const (
	StepUploadPrefix   = "upload_" // + split: upload_train, upload_val, upload_eval, upload_marker
	StepCreateDataset  = "create_dataset"
	StepSaveKernel     = "save_kernel"
	StepDownloadPrefix = "download_" // + kernel output file name, e.g. download_chibi-status.json
	StepDeleteDataset  = "delete_dataset"
	StepDeleteKernel   = "delete_kernel"
)

var (
	ErrRunActive      = errors.New("chibisensei: a training run is active")
	ErrRunNotTerminal = errors.New("chibisensei: the training run has not ended")
)

// maxRunEvents caps run_events per run; a variable so the cap test can lower it.
var maxRunEvents = 5000

// Leftover is a Kaggle resource a run left behind (cleanup off, delete not granted, or a failed delete).
type Leftover struct {
	Kind string `json:"kind"` // dataset | notebook
	Ref  string `json:"ref"`  // owner/slug
	URL  string `json:"url"`
}

// RunStage is the latest trainer stage seen in the log or the status file.
type RunStage struct {
	Name     string  `json:"name"`
	State    string  `json:"state"` // start | done | failed
	Step     int     `json:"step"`
	MaxSteps int     `json:"max_steps"`
	Epoch    float64 `json:"epoch"`
	EtaS     int     `json:"eta_s"`
}

// RunVersions are the software versions the trainer measured.
type RunVersions struct {
	Python       string `json:"python"`
	Torch        string `json:"torch"`
	Transformers string `json:"transformers"`
	PEFT         string `json:"peft"`
	LlamaCpp     string `json:"llama_cpp"`
}

// RunResult summarizes chibi-status.json once it was downloaded.
type RunResult struct {
	Steps         int          `json:"steps"`
	Epochs        float64      `json:"epochs"`
	TrainLoss     *float64     `json:"train_loss"`
	EvalLossBase  *float64     `json:"eval_loss_base"`
	EvalLossTuned *float64     `json:"eval_loss_tuned"`
	StopReason    string       `json:"stop_reason"`
	GPU           string       `json:"gpu"`
	Versions      *RunVersions `json:"versions,omitempty"` // always present in Run, dropped in RunSummary
}

// StagedFile is one dataset file staged in the Kaggle project (D12: the name inside the dataset equals Name).
type StagedFile struct {
	Split   string `json:"split"` // train | val | eval | marker
	Name    string `json:"name"`  // r<run12>-train.jsonl … r<run12>-marker.json
	SHA256  string `json:"sha256"`
	Bytes   int64  `json:"bytes"`
	Rows    int    `json:"rows"`
	Removed bool   `json:"removed"` // the staged copy was removed after create_dataset completed
}

// KaggleStep is the receipt of one Kaggle mutation of the run.
type KaggleStep struct {
	Step      string `json:"step"`                 // see the Step* constants
	Attempt   int    `json:"attempt"`              // attempt that produced RequestID (0-based)
	RequestID string `json:"request_id"`           // KaggleRequestID(run, step, attempt)
	JobID     string `json:"job_id"`               // "" until Execute admitted the request; then equal to RequestID
	Status    string `json:"status"`               // last Kaggle job status seen
	UploadRef string `json:"upload_ref,omitempty"` // upload steps: "upload:<job id>" from the completed receipt
	File      string `json:"file,omitempty"`       // upload: staged project file; download: kernel output path
	Error     string `json:"error,omitempty"`      // Job.Error (public text of the Kaggle package)
	Refused   bool   `json:"refused,omitempty"`    // Execute refused this attempt before admission
	Updated   string `json:"updated"`
}

// RunOutput is one file of the downloaded manifest.
type RunOutput struct {
	Path    string `json:"path"`
	Kind    string `json:"kind"`
	Bytes   int64  `json:"bytes"`
	SHA256  string `json:"sha256"`
	Copied  bool   `json:"copied"`            // verified and copied into Chibi storage
	Missing bool   `json:"missing,omitempty"` // optional output not available after the attempt limit
	Reused  bool   `json:"reused,omitempty"`  // the shared base GGUF was already stored; this run did not write it
}

// RunRemote holds everything of a run that has no column (runs.remote_json). Every field B4 needs to
// resume a state after a restart lives here.
type RunRemote struct {
	// Plan facts captured at submit.
	ProfileStatus         string    `json:"profile_status"`
	Method                string    `json:"method"`
	Split                 PlanSplit `json:"split"`
	EstimateMinutes       int       `json:"estimate_minutes"`
	SessionTimeoutMinutes int       `json:"session_timeout_minutes"`
	MaxTrainSeconds       int       `json:"max_train_seconds"`
	DataRev               int64     `json:"data_rev"`
	PlanHash              string    `json:"plan_hash"`
	MachineShape          string    `json:"machine_shape"`
	DeleteRemoteAfterRun  bool      `json:"delete_remote_after_run"` // the value the user confirmed

	// Destinations.
	Owner         string       `json:"owner"`
	DatasetSlug   string       `json:"dataset_slug"`   // without owner: chibi-data-<run12>
	NotebookSlug  string       `json:"notebook_slug"`  // without owner: chibi-<project8>-<run12>
	KaggleProject string       `json:"kaggle_project"` // local Kaggle project id (staging and downloads)
	Marker        string       `json:"marker"`         // marker file name (job.json data.marker)
	Staged        []StagedFile `json:"staged"`
	SnapshotBytes int64        `json:"snapshot_bytes"`

	// Kaggle receipts.
	Jobs          []KaggleStep `json:"jobs"`
	KernelVersion int          `json:"kernel_version"` // save_kernel versionNumber; 0 = unknown
	KernelID      int64        `json:"kernel_id"`
	RemoteStatus  string       `json:"remote_status"` // last Kaggle session status (upper case)
	Untracked     bool         `json:"untracked"`     // running without a usable save_kernel receipt (status reads only)

	// Timeline (RFC 3339; "" = not yet).
	Submitted          string `json:"submitted"`            // save_kernel admitted
	RemoteStarted      string `json:"remote_started"`       // first remote_running observation; the deadline is computed from it
	Finished           string `json:"finished"`             // terminal state reached
	DatasetWaitStarted string `json:"dataset_wait_started"` // waiting_dataset entered (15-minute cap)
	LastPolled         string `json:"last_polled"`          // last status poll
	LastLogRead        string `json:"last_log_read"`        // last ReadKernelLog call

	// Live progress.
	LastState  string    `json:"last_state"` // last non-terminal state; equals the state while the run is active
	Stage      *RunStage `json:"stage"`
	Live       string    `json:"live"`         // yes | no | unknown
	LastLogSeq int64     `json:"last_log_seq"` // highest @@chibi seq stored

	// Outcome.
	Result      *RunResult  `json:"result"`
	WarningCode string      `json:"warning_code"`
	Outputs     []RunOutput `json:"outputs"`

	// Cancel.
	CancelRemoteStopped bool   `json:"cancel_remote_stopped"`
	CancelError         string `json:"cancel_error"`

	// Attempts counts tries per state or step name (for example "downloading": at most 3).
	Attempts map[string]int `json:"attempts"`

	// Orchestrator facts (B4).
	Plan           *Plan             `json:"plan,omitempty"`            // the confirmed plan, stored by Submit
	EndState       string            `json:"end_state,omitempty"`       // succeeded | failed | cancelled once cleaning starts
	BlockedSince   string            `json:"blocked_since,omitempty"`   // first refused driver call of the current wait
	BlockedCode    string            `json:"blocked_code,omitempty"`    // kaggle_unavailable | desktop_readonly | kaggle_refused (one warn event per code)
	CancelTried    bool              `json:"cancel_tried,omitempty"`    // the remote cancel was tried (once)
	EmptyLogReads  int               `json:"empty_log_reads,omitempty"` // log reads without a @@chibi line in a row
	FinalLogRead   bool              `json:"final_log_read,omitempty"`  // downloading read the log once more
	OutputsPlanned bool              `json:"outputs_planned,omitempty"` // the manifest files are in Outputs
	Cleanup        map[string]string `json:"cleanup,omitempty"`         // staged | dataset | notebook → done | deleted | absent | kept | failed
	GGUF           *GGUFInfo         `json:"gguf,omitempty"`            // verified header of the tuned model
	Smoke          *SmokeResult      `json:"smoke,omitempty"`           // short generation check of the tuned model (verifying)
	ReportSeen     bool              `json:"report_seen,omitempty"`     // the report card of this succeeded run was opened
}

// Step returns the receipt of a step, or nil.
func (r *RunRemote) Step(name string) *KaggleStep {
	for i := range r.Jobs {
		if r.Jobs[i].Step == name {
			return &r.Jobs[i]
		}
	}
	return nil
}

// PutStep replaces the receipt with the same Step or appends it.
func (r *RunRemote) PutStep(s KaggleStep) {
	if cur := r.Step(s.Step); cur != nil {
		*cur = s
		return
	}
	r.Jobs = append(r.Jobs, s)
}

// Run is one training run. Its JSON form is the API shape "Run" (see MarshalJSON).
type Run struct {
	ID              string
	ProjectID       string
	Backend         string
	State           string
	StepAttempt     int
	ProfileID       string
	ProfileSHA      string
	Preset          string
	Hyper           Hyper
	JobJSON         []byte
	DataSHA         string
	Remote          RunRemote
	Deadline        string
	CancelRequested bool
	ErrorCode       string
	Error           string
	Leftovers       []Leftover
	Created         string
	Updated         string
}

// RunEvent is one row of run_events. Kind: state | stage | warn | error | kaggle | log.
type RunEvent struct {
	ID   int64           `json:"id"`
	TS   string          `json:"ts"`
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"data"`
}

// NewRunEvent marshals data (nil or an encoding error gives {}).
func NewRunEvent(kind string, data any) RunEvent {
	b, err := json.Marshal(data)
	if err != nil || data == nil {
		b = []byte("{}")
	}
	return RunEvent{Kind: kind, Data: b}
}

// Typed constructors for the event data shapes the UI reads.
func NewStateRunEvent(to string) RunEvent { return NewRunEvent("state", map[string]string{"to": to}) }
func NewStageRunEvent(stage, state string) RunEvent {
	return NewRunEvent("stage", map[string]string{"stage": stage, "state": state})
}
func NewWarnRunEvent(code, message string) RunEvent {
	return NewRunEvent("warn", map[string]string{"code": code, "message": message})
}
func NewErrorRunEvent(stage, code, message string) RunEvent {
	return NewRunEvent("error", map[string]string{"stage": stage, "code": code, "message": message})
}
func NewKaggleRunEvent(step, jobID string) RunEvent {
	return NewRunEvent("kaggle", map[string]string{"step": step, "job_id": jobID})
}
func NewLogRunEvent(text string) RunEvent { return NewRunEvent("log", map[string]string{"text": text}) }

// KaggleJobRef is one entry of remote.kaggle_jobs.
type KaggleJobRef struct {
	Step  string `json:"step"`
	JobID string `json:"job_id"`
}

// RunCancelView is the "cancel" object, present once cancel_requested is true.
type RunCancelView struct {
	RemoteStopped    bool   `json:"remote_stopped"`
	StopURL          string `json:"stop_url"` // "" before the notebook was submitted
	TimeLimitMinutes int    `json:"time_limit_minutes"`
}

// RunRemoteView is the "remote" object.
type RunRemoteView struct {
	Owner       string          `json:"owner"`
	Dataset     string          `json:"dataset"`  // owner/slug or ""
	Notebook    string          `json:"notebook"` // owner/slug or ""
	NotebookURL string          `json:"notebook_url"`
	Status      string          `json:"status"`
	Untracked   bool            `json:"untracked"`
	KaggleJobs  *[]KaggleJobRef `json:"kaggle_jobs,omitempty"` // Run only
}

// RunView is the API form of a run.
type RunView struct {
	ID                    string         `json:"id"`
	ProjectID             string         `json:"project_id"`
	Backend               string         `json:"backend"`
	State                 string         `json:"state"`
	LastState             string         `json:"last_state"`
	ProfileID             string         `json:"profile_id"`
	ProfileStatus         string         `json:"profile_status"`
	Preset                string         `json:"preset"`
	Method                string         `json:"method"`
	Hyper                 *Hyper         `json:"hyper,omitempty"` // Run only
	Split                 PlanSplit      `json:"split"`
	Stage                 *RunStage      `json:"stage"`
	Live                  string         `json:"live"`
	EstimateMinutes       int            `json:"estimate_minutes"`
	SessionTimeoutMinutes int            `json:"session_timeout_minutes"`
	Deadline              string         `json:"deadline"`
	RemoteStarted         string         `json:"remote_started"`
	Finished              string         `json:"finished"`
	CancelRequested       bool           `json:"cancel_requested"`
	Cancel                *RunCancelView `json:"cancel"`
	Remote                RunRemoteView  `json:"remote"`
	ErrorCode             string         `json:"error_code"`
	Error                 string         `json:"error"`
	WarningCode           string         `json:"warning_code"`
	Result                *RunResult     `json:"result"`
	Smoke                 *SmokeResult   `json:"smoke"`
	ReportSeen            bool           `json:"report_seen"`
	Leftovers             []Leftover     `json:"leftovers"`
	Created               string         `json:"created"`
	Updated               string         `json:"updated"`
}

// RunSummary is the list form: a RunView without hyper, remote.kaggle_jobs and result.versions.
type RunSummary RunView

func (r Run) view(full bool) RunView {
	rem := r.Remote
	v := RunView{ID: r.ID, ProjectID: r.ProjectID, Backend: r.Backend, State: r.State, LastState: rem.LastState, ProfileID: r.ProfileID,
		ProfileStatus: rem.ProfileStatus, Preset: r.Preset, Method: rem.Method, Split: rem.Split, Live: rem.Live,
		EstimateMinutes: rem.EstimateMinutes, SessionTimeoutMinutes: rem.SessionTimeoutMinutes, Deadline: r.Deadline,
		RemoteStarted: rem.RemoteStarted, Finished: rem.Finished, CancelRequested: r.CancelRequested,
		ErrorCode: r.ErrorCode, Error: r.Error, WarningCode: rem.WarningCode, Leftovers: r.Leftovers,
		Created: r.Created, Updated: r.Updated, ReportSeen: rem.ReportSeen}
	if rem.Smoke != nil {
		sm := *rem.Smoke
		v.Smoke = &sm
	}
	if v.Live == "" {
		v.Live = LiveUnknown
	}
	if v.Leftovers == nil {
		v.Leftovers = []Leftover{}
	}
	if rem.Stage != nil {
		st := *rem.Stage
		v.Stage = &st
	}
	dataset, notebook := KaggleRef(rem.Owner, rem.DatasetSlug), KaggleRef(rem.Owner, rem.NotebookSlug)
	v.Remote = RunRemoteView{Owner: rem.Owner, Dataset: dataset, Notebook: notebook, Status: rem.RemoteStatus,
		Untracked: rem.Untracked}
	if notebook != "" {
		v.Remote.NotebookURL = KaggleNotebookURL(notebook)
	}
	if r.CancelRequested {
		c := &RunCancelView{RemoteStopped: rem.CancelRemoteStopped, TimeLimitMinutes: rem.SessionTimeoutMinutes}
		if notebook != "" && (rem.Submitted != "" || rem.Untracked) {
			c.StopURL = KaggleNotebookURL(notebook)
		}
		v.Cancel = c
	}
	if rem.Result != nil {
		res := *rem.Result
		if full && res.Versions == nil {
			res.Versions = &RunVersions{}
		}
		if !full {
			res.Versions = nil
		}
		v.Result = &res
	}
	if full {
		h := r.Hyper
		v.Hyper = &h
		jobs := []KaggleJobRef{}
		for _, s := range rem.Jobs {
			if s.JobID != "" {
				jobs = append(jobs, KaggleJobRef{Step: s.Step, JobID: s.JobID})
			}
		}
		v.Remote.KaggleJobs = &jobs
	}
	return v
}

// View is the full API form ("Run").
func (r Run) View() RunView { return r.view(true) }

// Summary is the list form ("RunSummary").
func (r Run) Summary() RunSummary { return RunSummary(r.view(false)) }

// MarshalJSON writes the API shape "Run". Run has no UnmarshalJSON; decode API answers into RunView.
func (r Run) MarshalJSON() ([]byte, error) { return json.Marshal(r.View()) }

// KaggleRef joins owner and slug; "" when either is empty.
func KaggleRef(owner, slug string) string {
	if owner == "" || slug == "" {
		return ""
	}
	return owner + "/" + slug
}

// KaggleNotebookURL and KaggleDatasetURL are the public Kaggle pages of an owner/slug reference.
func KaggleNotebookURL(ref string) string { return "https://www.kaggle.com/code/" + ref }
func KaggleDatasetURL(ref string) string  { return "https://www.kaggle.com/datasets/" + ref }

// Artifact is one file a run registered under <data>/chibi_sensei.
type Artifact struct {
	ID        string `json:"id"`
	RunID     string `json:"-"`
	ProjectID string `json:"-"`
	Kind      string `json:"kind"`
	Name      string `json:"name"` // base name of RelPath; not stored
	RelPath   string `json:"-"`    // slash-separated, relative to <data>/chibi_sensei
	Bytes     int64  `json:"bytes"`
	SHA256    string `json:"sha256"`
	Created   string `json:"created"`
}

// ArtifactKinds are the manifest kinds plus log_tail.
var ArtifactKinds = []string{"gguf_tuned", "gguf_base", "adapter", "adapter_config", "metrics", "eval_samples", "status", "log_tail"}

func validArtifactKind(k string) bool {
	for _, x := range ArtifactKinds {
		if x == k {
			return true
		}
	}
	return false
}

// ArtifactID is the deterministic artifact ID of a run and kind (a UUID v5, so ValidID accepts it and a redone
// registration after a restart replaces the row instead of duplicating it).
func ArtifactID(runID, kind string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("aurago:chibi-sensei:artifact:"+runID+":"+kind)).String()
}
