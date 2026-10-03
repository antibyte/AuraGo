package memory

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/config"
	"aurago/internal/fileutil"

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
	Pinned     bool   `json:"pinned"`
	IsInternal bool   `json:"is_internal"`
	ID         int64  `json:"id"`
	Timestamp  string `json:"timestamp,omitempty"`
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
	Timestamp    string                   `json:"timestamp,omitempty"`
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
		Timestamp:    h.Timestamp,
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
	h.Timestamp = d.Timestamp
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
	closed         atomic.Bool      // Set by Close(); blocks Add and triggerSave
	isCompressing  bool             // Guard against concurrent compression
	saverWg        sync.WaitGroup   // Used by Close() to wait for backgroundSaver to finish
	loadErr        error            // Why the persisted history could not be used; guarded by mu
	persistBlocked atomic.Bool      // Set when the file could not be read or moved aside; disables saving
}

// readHistoryFile reads the persisted history. Tests replace it to simulate
// read failures that cannot be produced portably.
var readHistoryFile = os.ReadFile

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
		case <-hm.saveChan:
			if err := hm.save(); err != nil {
				slog.Error("Failed to save history to disk", "error", err)
			}
		}
	}
}

// Close stops the background saver goroutine and performs a final save.
func (hm *HistoryManager) Close() {
	hm.closeOnce.Do(func() {
		hm.mu.Lock()
		hm.closed.Store(true)
		hm.mu.Unlock()
		close(hm.doneChan)
		hm.saverWg.Wait()
		if err := hm.save(); err != nil {
			slog.Error("Failed to save history on close", "file", hm.file, "error", err)
		}
	})
}

// load restores the persisted history. Only a missing file means "no history".
// A read error leaves the file untouched and disables saving for this process;
// an unparseable or empty file is moved aside before a fresh history starts, so
// the next save can never destroy the only copy.
func (hm *HistoryManager) load() {
	if hm.file == "" {
		return // Ephemeral mode
	}
	hm.mu.Lock()
	defer hm.mu.Unlock()
	data, err := readHistoryFile(hm.file)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err != nil {
		hm.loadErr = fmt.Errorf("read chat history %s: %w; saving is disabled until the file is readable", hm.file, err)
		hm.persistBlocked.Store(true)
		slog.Error("Chat history unreadable; history persistence disabled to protect the file", "file", hm.file, "error", err)
		return
	}

	var parseErr error
	if len(bytes.TrimSpace(data)) == 0 {
		// save() never writes an empty file; zero bytes mean a crash or truncation.
		parseErr = errors.New("history file is empty")
	} else {
		var disk struct {
			Messages       []HistoryMessage `json:"messages"`
			CurrentSummary string           `json:"current_summary"`
		}
		// Decode into a local value so a type error cannot leave a half-filled history.
		if parseErr = json.Unmarshal(data, &disk); parseErr == nil {
			hm.Messages = disk.Messages
			if hm.Messages == nil {
				hm.Messages = []HistoryMessage{}
			}
			hm.CurrentSummary = disk.CurrentSummary
			return
		}
	}

	quarantine := fmt.Sprintf("%s.corrupt-%s-%s", hm.file, time.Now().UTC().Format("20060102T150405Z"), rand.Text()[:8])
	if renameErr := fileutil.Rename(hm.file, quarantine); renameErr != nil {
		hm.loadErr = fmt.Errorf("chat history %s is unreadable (%v) and could not be moved aside: %w; saving is disabled", hm.file, parseErr, renameErr)
		hm.persistBlocked.Store(true)
		slog.Error("Chat history unreadable and could not be moved aside; history persistence disabled", "file", hm.file, "parse_error", parseErr, "error", renameErr)
		return
	}
	hm.loadErr = fmt.Errorf("chat history %s was unreadable and moved to %s: %w", hm.file, quarantine, parseErr)
	slog.Error("Chat history unreadable; moved aside and starting a fresh history", "file", hm.file, "quarantine", quarantine, "error", parseErr)
}

func (hm *HistoryManager) triggerSave() {
	if hm.closed.Load() || hm.persistBlocked.Load() {
		return
	}
	select {
	case <-hm.doneChan:
		return
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
	if hm.persistBlocked.Load() {
		loadErr := hm.loadErr
		hm.mu.Unlock()
		return fmt.Errorf("chat history persistence disabled: %w", loadErr)
	}
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
	// Synced temp file in the same directory plus fileutil.Rename (Windows reader-lock
	// retries): a crash leaves either the previous or the new history, never a torn
	// or zero-length file. 0600: conversation history is sensitive.
	if err := config.WriteFileAtomic(hm.file, data, 0o600); err != nil {
		return fmt.Errorf("write chat history: %w", err)
	}
	return nil
}

// LoadError reports why the persisted chat history could not be used at startup:
// the file was unreadable (saving stays disabled) or corrupt (it was moved aside).
// It is nil after a normal load or when no history file existed.
func (hm *HistoryManager) LoadError() error {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	return hm.loadErr
}

// PersistenceBlocked reports whether saving is disabled to protect an unreadable file.
func (hm *HistoryManager) PersistenceBlocked() bool {
	return hm.persistBlocked.Load()
}

func (hm *HistoryManager) Add(role, content string, id int64, pinned bool, isInternal bool) error {
	hm.mu.Lock()
	if hm.closed.Load() {
		hm.mu.Unlock()
		return nil
	}
	hm.Messages = append(hm.Messages, HistoryMessage{
		ChatCompletionMessage: openai.ChatCompletionMessage{
			Role:    role,
			Content: content,
		},
		ID:         id,
		Pinned:     pinned,
		IsInternal: isInternal,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
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
	if hm.closed.Load() {
		hm.mu.Unlock()
		return nil
	}
	hm.Messages = append(hm.Messages, HistoryMessage{
		ChatCompletionMessage: msg,
		ID:                    id,
		Pinned:                pinned,
		IsInternal:            isInternal,
		Timestamp:             time.Now().UTC().Format(time.RFC3339),
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

	copied := make([]openai.ChatCompletionMessage, 0, len(hm.Messages))

	for _, stored := range hm.Messages {
		msg := stored.ChatCompletionMessage

		if len(msg.MultiContent) > 0 {
			msg.MultiContent = append([]openai.ChatMessagePart(nil), msg.MultiContent...)
		}
		if len(msg.ToolCalls) > 0 {
			msg.ToolCalls = append([]openai.ToolCall(nil), msg.ToolCalls...)
		}

		copied = append(copied, msg)
	}

	repaired, droppedCount := sanitizeToolMessagesForLLM(copied)
	if droppedCount > 0 {
		slog.Debug("GetForLLM: repaired message history", "dropped", droppedCount, "total_input", len(hm.Messages), "total_output", len(repaired))
	}

	return repaired
}

func sanitizeToolMessagesForLLM(msgs []openai.ChatCompletionMessage) ([]openai.ChatCompletionMessage, int) {
	if len(msgs) == 0 {
		return msgs, 0
	}

	dropped := 0
	repaired := make([]openai.ChatCompletionMessage, 0, len(msgs))

	for i := 0; i < len(msgs); i++ {
		msg := msgs[i]
		switch {
		case msg.Role == openai.ChatMessageRoleTool:
			dropped++
			continue
		case msg.Role != openai.ChatMessageRoleAssistant || len(msg.ToolCalls) == 0:
			repaired = append(repaired, msg)
			continue
		}

		expectedIDs := make(map[string]bool, len(msg.ToolCalls))
		for _, tc := range msg.ToolCalls {
			id := strings.TrimSpace(tc.ID)
			if id != "" {
				expectedIDs[id] = true
			}
		}
		if len(expectedIDs) == 0 {
			msg.ToolCalls = nil
			if strings.TrimSpace(msg.Content) == "" {
				dropped++
				continue
			}
			repaired = append(repaired, msg)
			continue
		}

		consumedIDs := make(map[string]bool, len(expectedIDs))
		toolResults := make([]openai.ChatCompletionMessage, 0, len(expectedIDs))
		j := i + 1
		for j < len(msgs) && msgs[j].Role == openai.ChatMessageRoleTool {
			toolMsg := msgs[j]
			id := strings.TrimSpace(toolMsg.ToolCallID)
			if id == "" || !expectedIDs[id] || consumedIDs[id] {
				dropped++
				j++
				continue
			}
			consumedIDs[id] = true
			toolResults = append(toolResults, toolMsg)
			j++
		}

		matched := make([]openai.ToolCall, 0, len(msg.ToolCalls))
		for _, tc := range msg.ToolCalls {
			id := strings.TrimSpace(tc.ID)
			if id != "" && consumedIDs[id] {
				matched = append(matched, tc)
			}
		}
		if len(matched) == 0 {
			if strings.TrimSpace(msg.Content) == "" {
				dropped++
			} else {
				msg.ToolCalls = nil
				repaired = append(repaired, msg)
			}
			i = j - 1
			continue
		}

		msg.ToolCalls = matched
		repaired = append(repaired, msg)
		repaired = append(repaired, toolResults...)
		i = j - 1
	}

	return repaired, dropped
}

func (hm *HistoryManager) GetAll() []HistoryMessage {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	copied := make([]HistoryMessage, len(hm.Messages))
	for i, m := range hm.Messages {
		msg := m.ChatCompletionMessage
		if len(msg.MultiContent) > 0 {
			msg.MultiContent = append([]openai.ChatMessagePart(nil), msg.MultiContent...)
		}
		if len(msg.ToolCalls) > 0 {
			msg.ToolCalls = append([]openai.ToolCall(nil), msg.ToolCalls...)
		}
		m.ChatCompletionMessage = msg
		copied[i] = m
	}
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

// ApplyCompression updates the persistent summary and removes exactly the
// selected stable message IDs under one lock and with one save notification.
// It returns the IDs that were still present and were actually removed.
func (hm *HistoryManager) ApplyCompression(summary string, ids []int64) ([]int64, error) {
	if hm == nil || strings.TrimSpace(summary) == "" || len(ids) == 0 {
		return nil, nil
	}
	idSet := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id > 0 {
			idSet[id] = struct{}{}
		}
	}
	if len(idSet) == 0 {
		return nil, nil
	}

	hm.mu.Lock()
	remaining := make([]HistoryMessage, 0, len(hm.Messages))
	dropped := make([]int64, 0, len(idSet))
	for _, message := range hm.Messages {
		if _, ok := idSet[message.ID]; ok && !message.Pinned {
			dropped = append(dropped, message.ID)
			continue
		}
		remaining = append(remaining, message)
	}
	if len(dropped) > 0 {
		hm.Messages = remaining
		hm.CurrentSummary = summary
	}
	hm.mu.Unlock()
	if len(dropped) > 0 {
		hm.triggerSave()
	}
	return dropped, nil
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
