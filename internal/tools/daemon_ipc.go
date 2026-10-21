package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"aurago/internal/security"
)

// Daemon IPC message types sent by skills (stdout → host).
const (
	DaemonMsgWakeAgent   = "wake_agent"
	DaemonMsgLog         = "log"
	DaemonMsgHeartbeat   = "heartbeat"
	DaemonMsgError       = "error"
	DaemonMsgShutdown    = "shutdown"
	DaemonMsgMetric      = "metric"
	DaemonMsgToolRequest = "tool_request"
)

// Daemon IPC command types sent by host (host → stdin).
const (
	DaemonCmdStop         = "stop"
	DaemonCmdConfigUpdate = "config_update"
	DaemonCmdToolResult   = "tool_result"
)

// DaemonMessage represents a JSON message from a daemon skill's stdout.
type DaemonMessage struct {
	Type      string          `json:"type"`
	Message   string          `json:"message,omitempty"`
	Severity  string          `json:"severity,omitempty"` // "info", "warning", "critical"
	Level     string          `json:"level,omitempty"`    // for log messages: "debug", "info", "warn", "error"
	Data      json.RawMessage `json:"data,omitempty"`
	Fatal     bool            `json:"fatal,omitempty"`  // for error messages
	Reason    string          `json:"reason,omitempty"` // for shutdown messages
	Timestamp int64           `json:"timestamp,omitempty"`
}

// DaemonCommand represents a JSON command from the host to a daemon skill's stdin.
type DaemonCommand struct {
	Type           string            `json:"type"`
	Reason         string            `json:"reason,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
	Env            map[string]string `json:"env,omitempty"` // for config_update
	Data           json.RawMessage   `json:"data,omitempty"`
}

// DaemonBridgeRun is created by the trusted server for one process generation.
// Its callback and credentials never cross the worker's stdin/stdout boundary.
type DaemonBridgeRun struct {
	Owner   context.Context
	Call    func(context.Context, DaemonToolRequest) DaemonToolResponse
	Release func()
}

type DaemonBridgeRunFactory func(context.Context, string, []string) (*DaemonBridgeRun, error)

type DaemonToolRequest struct {
	ID         uint64                 `json:"id"`
	Tool       string                 `json:"tool"`
	Parameters map[string]interface{} `json:"parameters"`
	Timeout    int                    `json:"timeout"`
}

type DaemonToolResponse struct {
	ID     uint64 `json:"id"`
	Status string `json:"status"`
	Result string `json:"result"`
}

// One writer and one request reader belong to this exact process generation.
// Closing stdin on cancellation also interrupts a worker that stops reading.
type daemonRunIPC struct {
	ctx      context.Context
	bridge   *DaemonBridgeRun
	commands chan DaemonCommand
	sequence uint64
}

func newDaemonRunIPC(ctx context.Context, pipe io.WriteCloser, bridge *DaemonBridgeRun) *daemonRunIPC {
	run := &daemonRunIPC{ctx: ctx, bridge: bridge, commands: make(chan DaemonCommand, 4)}
	stop := context.AfterFunc(ctx, func() { _ = pipe.Close() })
	go func() {
		defer stop()
		defer pipe.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case cmd := <-run.commands:
				if ctx.Err() != nil {
					return
				}
				data, err := EncodeDaemonCommand(cmd)
				if err != nil || len(data) > 1<<20 {
					return
				}
				if _, err := pipe.Write(append(data, '\n')); err != nil {
					return
				}
			}
		}
	}()
	return run
}

func (run *daemonRunIPC) send(cmd DaemonCommand) bool {
	if run == nil || run.ctx.Err() != nil {
		return false
	}
	select {
	case <-run.ctx.Done():
		return false
	case run.commands <- cmd:
		return true
	default:
		return false
	}
}

func (run *daemonRunIPC) toolRequest(data json.RawMessage) {
	if run == nil || run.ctx.Err() != nil || len(data) > 64<<10 {
		return
	}
	var request DaemonToolRequest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil || decoder.Decode(new(any)) != io.EOF || request.ID == 0 || request.ID != run.sequence+1 {
		return
	}
	run.sequence = request.ID
	response := DaemonToolResponse{ID: request.ID, Status: "error", Result: "Daemon tool bridge is unavailable"}
	if run.bridge != nil && run.bridge.Call != nil {
		timeout := min(60, max(1, request.Timeout))
		ctx, cancel := context.WithTimeout(run.ctx, time.Duration(timeout)*time.Second)
		response = func() (result DaemonToolResponse) {
			result = response
			defer func() {
				if recover() != nil {
					result = DaemonToolResponse{ID: request.ID, Status: "error", Result: "Tool outcome unknown; verify effects before another action"}
				}
			}()
			return run.bridge.Call(ctx, request)
		}()
		cancel()
	}
	if run.ctx.Err() != nil {
		return
	}
	response.ID = request.ID
	response.Result = security.Scrub(response.Result)
	encoded, err := json.Marshal(response)
	if err != nil || len(encoded) > (1<<20)-128 {
		encoded, _ = json.Marshal(DaemonToolResponse{ID: request.ID, Status: "error", Result: "Tool response exceeds the daemon IPC limit"})
	}
	run.send(DaemonCommand{Type: DaemonCmdToolResult, Data: encoded})
}

// DaemonStatus represents the lifecycle state of a daemon.
type DaemonStatus string

const (
	DaemonStopped  DaemonStatus = "stopped"
	DaemonStarting DaemonStatus = "starting"
	DaemonRunning  DaemonStatus = "running"
	DaemonCrashed  DaemonStatus = "crashed"
	DaemonDisabled DaemonStatus = "disabled"
)

// DaemonState holds the runtime state of a single daemon for serialization.
type DaemonState struct {
	SkillID         string       `json:"skill_id"`
	SkillName       string       `json:"skill_name"`
	Status          DaemonStatus `json:"status"`
	PID             int          `json:"pid,omitempty"`
	StartedAt       *time.Time   `json:"started_at,omitempty"`
	RestartCount    int          `json:"restart_count"`
	LastWakeUp      *time.Time   `json:"last_wake_up,omitempty"`
	WakeUpCount     int          `json:"wake_up_count"`
	SuppressedCount int          `json:"suppressed_count"`
	LastError       string       `json:"last_error,omitempty"`
	AutoDisabled    bool         `json:"auto_disabled"`
	LastActivity    *time.Time   `json:"last_activity,omitempty"`
}

// ParseDaemonMessage parses a single stdout line from a daemon skill into a DaemonMessage.
// Returns an error if the line is not valid JSON or missing the required "type" field.
func ParseDaemonMessage(line string) (DaemonMessage, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return DaemonMessage{}, fmt.Errorf("empty message line")
	}

	var msg DaemonMessage
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return DaemonMessage{}, fmt.Errorf("invalid daemon message JSON: %w", err)
	}

	if msg.Type == "" {
		return DaemonMessage{}, fmt.Errorf("daemon message missing required 'type' field")
	}

	switch msg.Type {
	case DaemonMsgWakeAgent, DaemonMsgLog, DaemonMsgHeartbeat, DaemonMsgError, DaemonMsgShutdown, DaemonMsgMetric, DaemonMsgToolRequest:
		// known types
	default:
		return DaemonMessage{}, fmt.Errorf("unknown daemon message type: %q", msg.Type)
	}

	// Default severity for wake_agent messages
	if msg.Type == DaemonMsgWakeAgent && msg.Severity == "" {
		msg.Severity = "info"
	}
	// Default level for log messages
	if msg.Type == DaemonMsgLog && msg.Level == "" {
		msg.Level = "info"
	}

	return msg, nil
}

// EncodeDaemonCommand serializes a host command for writing to a daemon skill's stdin.
// The output is a single JSON line (no trailing newline — caller must add \n).
func EncodeDaemonCommand(cmd DaemonCommand) ([]byte, error) {
	if cmd.Type == "" {
		return nil, fmt.Errorf("daemon command missing required 'type' field")
	}
	return json.Marshal(cmd)
}

// NewStopCommand creates a stop command with the given reason and grace period.
func NewStopCommand(reason string, gracePeriodSeconds int) DaemonCommand {
	return DaemonCommand{
		Type:           DaemonCmdStop,
		Reason:         reason,
		TimeoutSeconds: gracePeriodSeconds,
	}
}

// NewConfigUpdateCommand creates a config update command with new environment variables.
func NewConfigUpdateCommand(env map[string]string) DaemonCommand {
	return DaemonCommand{
		Type: DaemonCmdConfigUpdate,
		Env:  env,
	}
}
