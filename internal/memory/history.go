package memory

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/sashabaranov/go-openai"
)

type HistoryMessage struct {
	openai.ChatCompletionMessage
	Pinned     bool  `json:"pinned"`
	IsInternal bool  `json:"is_internal"`
	ID         int64 `json:"id"`
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
	if err == nil {
		_ = json.Unmarshal(data, hm)
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
	copy(snapshot.Messages, hm.Messages)
	hm.mu.Unlock()

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	// WriteFile is the slow part on Windows, now done completely outside the lock
	return os.WriteFile(hm.file, data, 0644)
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
	hm.mu.Unlock()

	hm.triggerSave()
	return nil
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
	hm.mu.Lock()
	defer hm.mu.Unlock()

	idMap := make(map[int64]bool)
	for _, id := range ids {
		idMap[id] = true
	}

	var remaining []HistoryMessage
	for _, m := range hm.Messages {
		if !idMap[m.ID] {
			remaining = append(remaining, m)
		}
	}
	hm.Messages = remaining
	hm.triggerSave()
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

func (hm *HistoryManager) TryLockCompression() bool {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	if hm.isCompressing {
		return false
	}
	hm.isCompressing = true
	return true
}

func (hm *HistoryManager) UnlockCompression() {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.isCompressing = false
}
