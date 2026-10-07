package flows

import "time"

// RunMode says why a run happened.
type RunMode string

const (
	ModeTest  RunMode = "test"
	ModeLive  RunMode = "live"
	ModeAgent RunMode = "agent"
	ModeCall  RunMode = "call"
)

// Valid reports whether m is one of the run modes above.
func (m RunMode) Valid() bool {
	switch m {
	case ModeTest, ModeLive, ModeAgent, ModeCall:
		return true
	}
	return false
}

// RunStatus is the status of a whole run.
type RunStatus string

const (
	RunQueued    RunStatus = "queued"
	RunRunning   RunStatus = "running"
	RunWaiting   RunStatus = "waiting"
	RunSuccess   RunStatus = "success"
	RunError     RunStatus = "error"
	RunCancelled RunStatus = "cancelled"
)

// Terminal reports whether the run is finished.
func (s RunStatus) Terminal() bool {
	return s == RunSuccess || s == RunError || s == RunCancelled
}

// Valid reports whether s is one of the run statuses above.
func (s RunStatus) Valid() bool {
	switch s {
	case RunQueued, RunRunning, RunWaiting, RunSuccess, RunError, RunCancelled:
		return true
	}
	return false
}

// StepStatus is the status of one node within a run.
type StepStatus string

const (
	StepPending   StepStatus = "pending"
	StepRunning   StepStatus = "running"
	StepSuccess   StepStatus = "success"
	StepError     StepStatus = "error"
	StepSkipped   StepStatus = "skipped"
	StepWaiting   StepStatus = "waiting"
	StepCancelled StepStatus = "cancelled"
)

// StepRecord is the outcome of one node in one run. Params (the resolved
// parameters) and Output are bounded: above MaxStoredOutputBytes they hold only
// {"_preview": "<start of the encoded JSON>"} and ParamsTruncated or
// OutputTruncated is set. Parameters that cannot be encoded are stored as
// {"_preview": "<unserializable>"} with ParamsTruncated set.
type StepRecord struct {
	NodeID          string         `json:"node_id"`
	NodeKey         string         `json:"node_key"`
	Attempt         int            `json:"attempt"`
	Status          StepStatus     `json:"status"`
	StartedAt       time.Time      `json:"started_at"`
	FinishedAt      time.Time      `json:"finished_at"`
	DurationMS      int64          `json:"duration_ms"`
	Params          map[string]any `json:"params,omitempty"`
	ParamsTruncated bool           `json:"params_truncated,omitempty"`
	Output          map[string]any `json:"output,omitempty"`
	OutputTruncated bool           `json:"output_truncated,omitempty"`
	ItemCount       int            `json:"item_count,omitempty"`
	Ports           []string       `json:"ports,omitempty"`
	ErrorCode       string         `json:"error_code,omitempty"`
	ErrorMessage    string         `json:"error_message,omitempty"`
}

// Run event types.
const (
	EventRunStarted   = "run_started"
	EventStepStarted  = "step_started"
	EventStepFinished = "step_finished"
	EventRunFinished  = "run_finished"
)

// RunEvent is one entry of a run's live event stream. Seq starts at 1 per run.
type RunEvent struct {
	Seq    int         `json:"seq"`
	RunID  string      `json:"run_id"`
	Type   string      `json:"type"`
	NodeID string      `json:"node_id,omitempty"`
	Step   *StepRecord `json:"step,omitempty"`
	Run    *RunSummary `json:"run,omitempty"`
	Time   time.Time   `json:"time"`
}

// RunSummary describes the run in run_started / run_finished events.
type RunSummary struct {
	Status       RunStatus `json:"status"`
	ErrorCode    string    `json:"error_code,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
	DurationMS   int64     `json:"duration_ms,omitempty"`
}

// RunResult is what the engine returns for a finished run.
type RunResult struct {
	Status       RunStatus
	ErrorCode    string
	ErrorMessage string
	ErrorNodeID  string
	StartedAt    time.Time
	FinishedAt   time.Time
	DurationMS   int64
	Steps        []StepRecord
	Outputs      map[string]map[string]any
}

// RunRecord is the persisted header of a run.
//
// TriggerData is the trigger's data object. Of the records read from the store
// only GetRun fills it; ListRuns and LastLiveRuns leave it empty because the
// payload can be large. The store keeps at most MaxStoredOutputBytes of it: a
// larger payload is stored as {"_preview": "<start of the encoded JSON>"}.
type RunRecord struct {
	ID           string         `json:"id"`
	FlowID       string         `json:"flow_id"`
	Revision     int            `json:"revision"`
	Mode         RunMode        `json:"mode"`
	TriggerNode  string         `json:"trigger_node"`
	TriggerType  string         `json:"trigger_type"`
	TriggerData  map[string]any `json:"trigger_data,omitempty"`
	Status       RunStatus      `json:"status"`
	ErrorCode    string         `json:"error_code,omitempty"`
	ErrorMessage string         `json:"error_message,omitempty"`
	StartedAt    time.Time      `json:"started_at"`
	FinishedAt   *time.Time     `json:"finished_at,omitempty"`
	DurationMS   int64          `json:"duration_ms"`
	ParentRunID  string         `json:"parent_run_id,omitempty"`
	ParentNodeID string         `json:"parent_node_id,omitempty"`
}
