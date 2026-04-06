package tools

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Daemon IPC message types sent by skills (stdout → host).
const (
	DaemonMsgWakeAgent = "wake_agent"
	DaemonMsgLog       = "log"
	DaemonMsgHeartbeat = "heartbeat"
	DaemonMsgError     = "error"
	DaemonMsgShutdown  = "shutdown"
)

// Daemon IPC command types sent by host (host → stdin).
const (
	DaemonCmdStop         = "stop"
	DaemonCmdConfigUpdate = "config_update"
)

// DaemonMessage represents a JSON message from a daemon skill's stdout.
type DaemonMessage struct {
	Type      string          `json:"type"`
	Message   string          `json:"message,omitempty"`
	Severity  string          `json:"severity,omitempty"` // "info", "warning", "critical"
	Level     string          `json:"level,omitempty"`    // for log messages: "debug", "info", "warn", "error"
	Data      json.RawMessage `json:"data,omitempty"`
	Fatal     bool            `json:"fatal,omitempty"`    // for error messages
	Reason    string          `json:"reason,omitempty"`   // for shutdown messages
	Timestamp int64           `json:"timestamp,omitempty"`
}

// DaemonCommand represents a JSON command from the host to a daemon skill's stdin.
type DaemonCommand struct {
	Type           string            `json:"type"`
	Reason         string            `json:"reason,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
	Env            map[string]string `json:"env,omitempty"` // for config_update
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
	case DaemonMsgWakeAgent, DaemonMsgLog, DaemonMsgHeartbeat, DaemonMsgError, DaemonMsgShutdown:
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
