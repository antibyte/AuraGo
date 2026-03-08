package webhooks

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// LogEntry records a single webhook invocation for debugging.
type LogEntry struct {
	Timestamp   time.Time `json:"timestamp"`
	WebhookID   string    `json:"webhook_id"`
	WebhookName string    `json:"webhook_name"`
	StatusCode  int       `json:"status_code"`
	SourceIP    string    `json:"source_ip"`
	PayloadSize int       `json:"payload_size"`
	Delivered   bool      `json:"delivered"`
	Error       string    `json:"error,omitempty"`
}

// Log is a ring-buffer backed webhook event log.
type Log struct {
	mu       sync.Mutex
	filePath string
	maxSize  int
	entries  []LogEntry
}

// NewLog creates a Log that persists at filePath with at most maxSize entries.
func NewLog(filePath string, maxSize int) (*Log, error) {
	l := &Log{filePath: filePath, maxSize: maxSize}
	if err := l.load(); err != nil {
		l.entries = []LogEntry{}
	}
	return l, nil
}

func (l *Log) load() error {
	data, err := os.ReadFile(l.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			l.entries = []LogEntry{}
			return nil
		}
		return err
	}
	if len(data) == 0 {
		l.entries = []LogEntry{}
		return nil
	}
	return json.Unmarshal(data, &l.entries)
}

func (l *Log) save() error {
	data, err := json.MarshalIndent(l.entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(l.filePath, data, 0644)
}

// Append adds an entry and trims to maxSize.
func (l *Log) Append(entry LogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.entries = append(l.entries, entry)
	if len(l.entries) > l.maxSize {
		l.entries = l.entries[len(l.entries)-l.maxSize:]
	}
	_ = l.save()
}

// Recent returns the latest n entries (newest first).
func (l *Log) Recent(n int) []LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	if n <= 0 || len(l.entries) == 0 {
		return []LogEntry{}
	}
	if n > len(l.entries) {
		n = len(l.entries)
	}
	// Return newest first
	result := make([]LogEntry, n)
	for i := 0; i < n; i++ {
		result[i] = l.entries[len(l.entries)-1-i]
	}
	return result
}

// ForWebhook returns recent entries for a specific webhook.
func (l *Log) ForWebhook(webhookID string, n int) []LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	var result []LogEntry
	for i := len(l.entries) - 1; i >= 0 && len(result) < n; i-- {
		if l.entries[i].WebhookID == webhookID {
			result = append(result, l.entries[i])
		}
	}
	return result
}
