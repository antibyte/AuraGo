package memory

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sashabaranov/go-openai"
)

// maxEphemeralMessages is the upper bound on the number of messages held by an
// ephemeral (co-agent) HistoryManager. When the limit is reached the oldest
// non-pinned messages are trimmed to 75 % of the limit.
const (
	maxEphemeralMessages = 200
)

type HistoryMessage struct {
	openai.ChatCompletionMessage
	Pinned     bool  `json:"pinned"`
	IsInternal bool  `json:"is_internal"`
	ID         int64 `json:"id"`
}

// historyMessageDisk is the on-disk/JSON representation of HistoryMessage.
// It exists because openai.ChatCompletionMessage has custom MarshalJSON/UnmarshalJSON
// methods that, when promoted via embedding, silently drop the Pinned, IsInternal,
// and ID fields. By defining explicit methods on HistoryMessage we take full control.
type historyMessageDisk struct {
	Role         string                   `json:"role"`
	Content      string                   `json:"content,omitempty"`
	MultiContent []openai.ChatMessagePart `json:"multi_content,omitempty"`
	Name         string                   `json:"name,omitempty"`
	FunctionCall *openai.FunctionCall     `json:"function_call,omitempty"`
	ToolCalls    []openai.ToolCall        `json:"tool_calls,omitempty"`
	ToolCallID   string                   `json:"tool_call_id,omitempty"`
	Pinned       bool                     `json:"pinned"`
	IsInternal   bool                     `json:"is_internal"`
	ID           int64                    `json:"id"`
}

// MarshalJSON serialises all fields including Pinned, IsInternal, and ID.
func (h HistoryMessage) MarshalJSON() ([]byte, error) {
	content := h.Content
	multi := h.MultiContent
	if len(multi) > 0 {
		// openai.ChatCompletionMessage rejects having both Content and MultiContent set.
		content = ""
	}
	return json.Marshal(historyMessageDisk{
		Role:         h.Role,
		Content:      content,
		MultiContent: multi,
		Name:         h.Name,
		FunctionCall: h.FunctionCall,
		ToolCalls:    h.ToolCalls,
		ToolCallID:   h.ToolCallID,
		Pinned:       h.Pinned,
		IsInternal:   h.IsInternal,
		ID:           h.ID,
	})
}

// UnmarshalJSON restores all fields including Pinned, IsInternal, and ID.
func (h *HistoryMessage) UnmarshalJSON(data []byte) error {
	var d historyMessageDisk
	if err := json.Unmarshal(data, &d); err != nil {
		return err
	}
	h.ChatCompletionMessage = openai.ChatCompletionMessage{
		Role:         d.Role,
		Content:      d.Content,
		MultiContent: d.MultiContent,
		Name:         d.Name,
		FunctionCall: d.FunctionCall,
		ToolCalls:    d.ToolCalls,
		ToolCallID:   d.ToolCallID,
	}
	h.Pinned = d.Pinned
	h.IsInternal = d.IsInternal
	h.ID = d.ID
	return nil
}

type HistoryManager struct {
	mu             sync.Mutex
	file           string
	Messages       []HistoryMessage `json:"messages"`
	CurrentSummary string           `json:"current_summary"`
	saveChan       chan struct{}    // Notify background saver
	doneChan       chan struct{}    // Signals backgroundSaver to exit
	closeOnce      sync.Once        // Prevents double-close panic on doneChan
	isCompressing  bool             // Guard against concurrent compression
	saverWg        sync.WaitGroup   // Used by Close() to wait for backgroundSaver to finish
}

func NewHistoryManager(filePath string) *HistoryManager {
	hm := &HistoryManager{
		file:     filePath,
		Messages: []HistoryMessage{},
		saveChan: make(chan struct{}, 1),
		doneChan: make(chan struct{}),
	}
	hm.load()

	// Start background saver
	hm.saverWg.Add(1)
	go hm.backgroundSaver()

	return hm
}

// NewEphemeralHistoryManager creates an in-memory-only HistoryManager.
// Used by co-agents — no disk persistence, no compression.
func NewEphemeralHistoryManager() *HistoryManager {
	return &HistoryManager{
		file:     "",
		Messages: []HistoryMessage{},
		saveChan: make(chan struct{}, 1),
		doneChan: make(chan struct{}),
	}
}

func (hm *HistoryManager) backgroundSaver() {
	defer hm.saverWg.Done()
	for {
		select {
		case <-hm.doneChan:
			return
		case _, ok := <-hm.saveChan:
			if !ok {
				return
			}
			if err := hm.save(); err != nil {
				slog.Error("Failed to save history to disk", "error", err)
			}
		}
	}
}

// Close stops the background saver goroutine and performs a final save.
func (hm *HistoryManager) Close() {
	hm.closeOnce.Do(func() {
		close(hm.doneChan)
		hm.saverWg.Wait() // wait for backgroundSaver to drain before final save
		hm.save()
	})
}

func (hm *HistoryManager) load() {
	if hm.file == "" {
		return // Ephemeral mode
	}
	hm.mu.Lock()
	defer hm.mu.Unlock()
	data, err := os.ReadFile(hm.file)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("Failed to read history file, starting fresh", "file", hm.file, "error", err)
		}
		return
	}
	if len(data) == 0 {
		// Empty file is fine, start with empty history
		return
	}
	if err := json.Unmarshal(data, hm); err != nil {
		slog.Error("Failed to parse history file, starting fresh", "file", hm.file, "error", err)
		hm.Messages = nil
		hm.CurrentSummary = ""
	}
}

func (hm *HistoryManager) triggerSave() {
	select {
	case hm.saveChan <- struct{}{}:
	default:
		// Save already pending
	}
}

func (hm *HistoryManager) save() error {
	if hm.file == "" {
		return nil // Ephemeral mode — no disk persistence
	}
	hm.mu.Lock()
	// Deep-copy the data under lock so we can release it before the expensive marshal+write
	snapshot := &HistoryManager{
		Messages:       make([]HistoryMessage, len(hm.Messages)),
		CurrentSummary: hm.CurrentSummary,
	}
	// Deep-copy each message so ToolCalls slices are not shared with the live slice.
	for i, m := range hm.Messages {
		msg := m
		if len(m.ToolCalls) > 0 {
			msg.ToolCalls = make([]openai.ToolCall, len(m.ToolCalls))
			copy(msg.ToolCalls, m.ToolCalls)
		}
		snapshot.Messages[i] = msg
	}
	hm.mu.Unlock()

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	// Atomic write via temp file + rename to prevent corruption on crash.
	// The temp file is created in the same directory to guarantee the rename
	// is atomic (same filesystem on all platforms).
	dir := filepath.Dir(hm.file)
	tmpFile, err := os.CreateTemp(dir, ".history-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmpFile.Name()

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp file: %w", err)
	}
	// 0600: owner read/write only — conversation history is sensitive.
	if err := tmpFile.Chmod(0600); err != nil {
		tmpFile.Close()
		os.Remove(tmpName)
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, hm.file); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename temp→final: %w", err)
	}
	return nil
}

func (hm *HistoryManager) Add(role, content string, id int64, pinned bool, isInternal bool) error {
	hm.mu.Lock()
	hm.Messages = append(hm.Messages, HistoryMessage{
		ChatCompletionMessage: openai.ChatCompletionMessage{
			Role:    role,
			Content: content,
		},
		ID:         id,
		Pinned:     pinned,
		IsInternal: isInternal,
	})
	// For ephemeral (co-agent) history, enforce a message-count ceiling to
	// prevent unbounded memory growth in long-running co-agent loops.
	if hm.file == "" && len(hm.Messages) > maxEphemeralMessages {
		hm.trimEphemeralLocked()
	}
	hm.mu.Unlock()

	hm.triggerSave()
	return nil
}

func (hm *HistoryManager) AddMessage(msg openai.ChatCompletionMessage, id int64, pinned bool, isInternal bool) error {
	hm.mu.Lock()
	hm.Messages = append(hm.Messages, HistoryMessage{
		ChatCompletionMessage: msg,
		ID:                    id,
		Pinned:                pinned,
		IsInternal:            isInternal,
	})
	// For ephemeral (co-agent) history, enforce a message-count ceiling to
	// prevent unbounded memory growth in long-running co-agent loops.
	if hm.file == "" && len(hm.Messages) > maxEphemeralMessages {
		hm.trimEphemeralLocked()
	}
	hm.mu.Unlock()

	hm.triggerSave()
	return nil
}

// trimEphemeralLocked removes the oldest non-pinned messages until the slice is
// at 75 % of maxEphemeralMessages. Must be called with hm.mu held.
// If all messages are pinned and the slice still exceeds the target, the oldest
// messages are force-removed regardless of pin status to prevent unbounded growth.
func (hm *HistoryManager) trimEphemeralLocked() {
	targetLen := maxEphemeralMessages * 3 / 4
	if len(hm.Messages) <= targetLen {
		return
	}
	result := make([]HistoryMessage, 0, targetLen+16)
	toRemove := len(hm.Messages) - targetLen
	removed := 0
	for _, m := range hm.Messages {
		if removed < toRemove && !m.Pinned {
			removed++
			continue
		}
		result = append(result, m)
	}
	// Hard cap: if all messages were pinned and nothing was removed, force-trim
	// the oldest entries to prevent unbounded memory growth in co-agent loops.
	if removed == 0 && len(result) > targetLen {
		result = result[len(result)-targetLen:]
	}
	hm.Messages = result
}

func (hm *HistoryManager) SetPinned(id int64, pinned bool) error {
	hm.mu.Lock()
	found := false
	for i := range hm.Messages {
		if hm.Messages[i].ID == id {
			hm.Messages[i].Pinned = pinned
			found = true
			break
		}
	}
	hm.mu.Unlock()

	if !found {
		return os.ErrNotExist
	}

	hm.triggerSave()
	return nil
}

func (hm *HistoryManager) Get() []openai.ChatCompletionMessage {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	copied := make([]openai.ChatCompletionMessage, len(hm.Messages))
	for i, m := range hm.Messages {
		msg := m.ChatCompletionMessage
		if len(msg.MultiContent) > 0 {
			msg.MultiContent = append([]openai.ChatMessagePart(nil), msg.MultiContent...)
		}
		if len(msg.ToolCalls) > 0 {
			msg.ToolCalls = append([]openai.ToolCall(nil), msg.ToolCalls...)
		}
		copied[i] = msg
	}
	return copied
}

// GetForLLM returns a sanitized message history that is safe to send to OpenAI-style
// providers with native tool calling enabled.
//
// It removes dangling role=tool messages that would cause 400 errors like
// "tool_call_id not found" / "tool result's tool id not found" when:
// - tool results are persisted without ToolCallID, or
// - tool results no longer match any preceding assistant tool_calls (e.g. after restart).
//
// The repair is conservative: it never mutates hm.Messages; it only filters/normalizes
// the returned slice.
func (hm *HistoryManager) GetForLLM() []openai.ChatCompletionMessage {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	var repaired []openai.ChatCompletionMessage
	droppedCount := 0

	outstanding := make(map[string]struct{})
	inToolRound := false

	closeToolRound := func() {
		if inToolRound {
			inToolRound = false
			clear(outstanding)
		}
	}

	for _, stored := range hm.Messages {
		msg := stored.ChatCompletionMessage

		if len(msg.MultiContent) > 0 {
			msg.MultiContent = append([]openai.ChatMessagePart(nil), msg.MultiContent...)
		}
		if len(msg.ToolCalls) > 0 {
			msg.ToolCalls = append([]openai.ToolCall(nil), msg.ToolCalls...)
		}

		switch msg.Role {
		case openai.ChatMessageRoleTool:
			if !inToolRound {
				droppedCount++
				continue
			}
			id := strings.TrimSpace(msg.ToolCallID)
			if id == "" {
				droppedCount++
				continue
			}
			if _, ok := outstanding[id]; !ok {
				droppedCount++
				continue
			}
			delete(outstanding, id)
			repaired = append(repaired, msg)
		case openai.ChatMessageRoleAssistant:
			closeToolRound()

			if len(msg.ToolCalls) > 0 {
				valid := true
				for _, tc := range msg.ToolCalls {
					if strings.TrimSpace(tc.ID) == "" {
						valid = false
						break
					}
				}
				if !valid {
					msg.ToolCalls = nil
				}
			}

			repaired = append(repaired, msg)
			if len(msg.ToolCalls) > 0 {
				inToolRound = true
				for _, tc := range msg.ToolCalls {
					outstanding[strings.TrimSpace(tc.ID)] = struct{}{}
				}
			}
		default:
			closeToolRound()
			repaired = append(repaired, msg)
		}
	}

	// Safety net: if there are still outstanding (unmatched) tool-call IDs after the
	// loop, filter out only the unmatched tool calls from the last assistant message.
	// Only drop the entire assistant message if all of its tool calls are unmatched
	// and it has no content.
	if len(outstanding) > 0 {
		for i := len(repaired) - 1; i >= 0; i-- {
			if repaired[i].Role == openai.ChatMessageRoleAssistant && len(repaired[i].ToolCalls) > 0 {
				matched := make([]openai.ToolCall, 0, len(repaired[i].ToolCalls))
				for _, tc := range repaired[i].ToolCalls {
					if _, ok := outstanding[strings.TrimSpace(tc.ID)]; !ok {
						matched = append(matched, tc)
					}
				}
				repaired[i].ToolCalls = matched
				if len(repaired[i].ToolCalls) == 0 && strings.TrimSpace(repaired[i].Content) == "" {
					repaired = append(repaired[:i], repaired[i+1:]...)
					droppedCount++
				}
				break
			}
		}
	}

	if droppedCount > 0 {
		slog.Debug("GetForLLM: repaired message history", "dropped", droppedCount, "total_input", len(hm.Messages), "total_output", len(repaired))
	}

	return repaired
}

func (hm *HistoryManager) GetAll() []HistoryMessage {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	copied := make([]HistoryMessage, len(hm.Messages))
	copy(copied, hm.Messages)
	return copied
}

func (hm *HistoryManager) GetSummary() string {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	return hm.CurrentSummary
}

func (hm *HistoryManager) SetSummary(summary string) error {
	hm.mu.Lock()
	hm.CurrentSummary = summary
	hm.mu.Unlock()

	hm.triggerSave()
	return nil
}

func (hm *HistoryManager) DropFirstN(n int) error {
	hm.mu.Lock()
	if n >= len(hm.Messages) {
		hm.Messages = []HistoryMessage{}
	} else {
		hm.Messages = hm.Messages[n:]
	}
	hm.mu.Unlock()

	hm.triggerSave()
	return nil
}

func (hm *HistoryManager) Clear() error {
	hm.mu.Lock()
	hm.Messages = []HistoryMessage{}
	hm.CurrentSummary = ""
	hm.mu.Unlock()

	hm.triggerSave()
	return nil
}

// TotalChars returns the total character count of all stored messages.
func (hm *HistoryManager) TotalChars() int {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	total := 0
	for _, m := range hm.Messages {
		total += len(m.Content)
	}
	return total
}

// GetOldestMessagesForPruning returns the first N messages that sum up to at least targetChars,
// skipping pinned messages. It also returns the actual character count of those messages.
func (hm *HistoryManager) GetOldestMessagesForPruning(targetChars int) ([]HistoryMessage, int) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	var prunedMsgs []HistoryMessage
	currentChars := 0

	for _, m := range hm.Messages {
		if m.Pinned {
			continue
		}
		if currentChars >= targetChars {
			break
		}
		prunedMsgs = append(prunedMsgs, m)
		currentChars += len(m.Content)
	}

	return prunedMsgs, currentChars
}

// DropMessages removes the specified messages from the history by their IDs.
func (hm *HistoryManager) DropMessages(ids []int64) {
	if len(ids) == 0 {
		return
	}
	hm.mu.Lock()
	defer hm.mu.Unlock()

	idMap := make(map[int64]bool)
	for _, id := range ids {
		idMap[id] = true
	}

	var remaining []HistoryMessage
	dropped := 0
	for _, m := range hm.Messages {
		if idMap[m.ID] {
			dropped++
			continue
		}
		remaining = append(remaining, m)
	}
	hm.Messages = remaining
	if dropped > 0 {
		hm.triggerSave()
	}
}

// TotalPinnedChars returns the total character count of all pinned messages.
func (hm *HistoryManager) TotalPinnedChars() int {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	total := 0
	for _, m := range hm.Messages {
		if m.Pinned {
			total += len(m.Content)
		}
	}
	return total
}

// TryLockCompression attempts to acquire the compression lock.
// Returns true and a release function if lock was acquired (caller MUST defer release).
// Returns false if compression is already in progress.
// The release function resets the lock and MUST be called even on error.
func (hm *HistoryManager) TryLockCompression() (bool, func()) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	if hm.isCompressing {
		return false, nil
	}
	hm.isCompressing = true
	return true, func() {
		hm.mu.Lock()
		hm.isCompressing = false
		hm.mu.Unlock()
	}
}
