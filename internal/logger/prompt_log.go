package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const promptLogFileName = "prompts.log"

// PromptLogPath returns the canonical path for verbose prompt logging.
func PromptLogPath(logDir string) string {
	return filepath.Join(logDir, promptLogFileName)
}

// ResetPromptLog truncates the prompt log at startup so it contains only the
// current process run. The directory is created independently of file logging.
func ResetPromptLog(logDir string) error {
	if strings.TrimSpace(logDir) == "" {
		return fmt.Errorf("prompt log directory is required")
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("create prompt log directory: %w", err)
	}
	if err := os.WriteFile(PromptLogPath(logDir), nil, 0600); err != nil {
		return fmt.Errorf("truncate prompt log: %w", err)
	}
	return nil
}

// AppendPromptLogEntry appends one JSON line to prompts.log.
func AppendPromptLogEntry(logDir string, entry any) error {
	if strings.TrimSpace(logDir) == "" {
		return fmt.Errorf("prompt log directory is required")
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("create prompt log directory: %w", err)
	}

	f, err := os.OpenFile(PromptLogPath(logDir), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open prompt log: %w", err)
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(entry); err != nil {
		return fmt.Errorf("encode prompt log entry: %w", err)
	}
	return nil
}
