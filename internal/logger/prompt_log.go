package logger

import (
	"aurago/internal/security"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const promptLogFileName = "prompts.log"

var promptLogMu sync.Mutex

// PromptLogPath returns the canonical path for verbose prompt logging.
func PromptLogPath(logDir string) string {
	return filepath.Join(logDir, promptLogFileName)
}

// ResetPromptLog truncates the prompt log at startup so it contains only the
// current process run. The directory is created independently of file logging.
func ResetPromptLog(logDir string) error {
	promptLogMu.Lock()
	defer promptLogMu.Unlock()
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

	promptLogMu.Lock()
	defer promptLogMu.Unlock()
	// Scrub strings before encoding so quotes, backslashes and newlines in a
	// registered secret cannot bypass redaction through JSON escaping.
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("encode prompt log entry: %w", err)
	}
	var value any
	if err = json.Unmarshal(data, &value); err != nil {
		return err
	}
	data, err = json.Marshal(scrubPromptValue(value))
	if err != nil {
		return err
	}
	if int64(len(data)+1) > defaultLogBytes {
		data = []byte(`{"omitted":true,"reason":"prompt log entry exceeds file size limit"}`)
	}
	f, err := newRotatingLog(PromptLogPath(logDir), true, defaultLogBytes, defaultLogBackups)
	if err != nil {
		return fmt.Errorf("open prompt log: %w", err)
	}
	defer f.Close()
	_, err = f.Write(append(data, '\n'))
	return err
}
func scrubPromptValue(value any) any {
	switch v := value.(type) {
	case string:
		return security.Scrub(v)
	case []any:
		for i := range v {
			v[i] = scrubPromptValue(v[i])
		}
	case map[string]any:
		scrubbed := make(map[string]any, len(v))
		for key, item := range v {
			scrubbed[security.Scrub(key)] = scrubPromptValue(item)
		}
		return scrubbed
	}
	return value
}
