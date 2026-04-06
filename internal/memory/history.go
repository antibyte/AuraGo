package memory

import (
	"encoding/json"
	"log/slog"
	"os"
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
	Role         string               `json:"role"`
	Content      string               `json:"content,omitempty"`
	Name         string               `json:"name,omitempty"`
	FunctionCall *openai.FunctionCall `json:"function_call,omitempty"`
	ToolCalls    []openai.ToolCall    `json:"tool_calls,omitempty"`
	ToolCallID   string               `json:"tool_call_id,omitempty"`
	Pinned       bool                 `json:"pinned"`
	IsInternal   bool                 `json:"is_internal"`
	ID           int64                `json:"id"`
}

// MarshalJSON serialises all fields including Pinned, IsInternal, and ID.
func (h HistoryMessage) MarshalJSON() ([]byte, error) {
	return json.Marshal(historyMessageDisk{
		Role:         h.Role,
		Content:      h.Content,
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
			hm.save()
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
	// WriteFile is the slow part on Windows, now done completely outside the lock.
	// 0600: owner read/write only — conversation history is sensitive.
	// On Windows the ACL model differs and mode is advisory; the file will be created
	// with default ACLs inherited from the parent directory.
	return os.WriteFile(hm.file, data, 0600)
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
		copied[i] = m.ChatCompletionMessage
	}
	return copied
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
