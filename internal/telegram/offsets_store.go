package telegram

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const telegramOffsetsFileName = "telegram_offsets.json"

// OffsetStore persists the Telegram update cursor so a restart does not
// replay updates that were already handed to a worker.
type OffsetStore interface {
	Load(botID int64) (int, bool)
	Save(botID int64, offset int) error
}

// FileOffsetStore keeps the Telegram update cursor in a JSON file inside the
// data directory. Writes go to a temporary file in the same directory and are
// renamed into place, so a crash never leaves a half-written cursor behind.
type FileOffsetStore struct {
	path   string
	mu     sync.Mutex
	logger *slog.Logger
}

// telegramOffsetFile is the on-disk cursor document.
type telegramOffsetFile struct {
	Offsets map[string]int `json:"offsets"`
}

// NewFileOffsetStore binds a cursor file to the data directory. An empty data
// directory yields no store, which keeps the cursor in memory only.
func NewFileOffsetStore(dataDir string) (*FileOffsetStore, error) {
	dir := strings.TrimSpace(dataDir)
	if dir == "" {
		return nil, nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create telegram cursor directory: %w", err)
	}
	return &FileOffsetStore{path: filepath.Join(dir, telegramOffsetsFileName)}, nil
}

// SetLogger attaches the logger used for cursor file warnings.
func (s *FileOffsetStore) SetLogger(logger *slog.Logger) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger = logger
}

// Path returns the cursor file location.
func (s *FileOffsetStore) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// Load returns the persisted cursor for botID. A missing or unreadable cursor
// file is reported as "no cursor" so the bot still starts at offset 0.
func (s *FileOffsetStore) Load(botID int64) (int, bool) {
	if s == nil {
		return 0, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	offsets, err := s.read()
	if err != nil {
		s.warn("Failed to read Telegram cursor file", err)
		return 0, false
	}
	offset, ok := offsets[strconv.FormatInt(botID, 10)]
	return offset, ok
}

// Save persists the cursor for botID, keeping the cursors of other bots.
func (s *FileOffsetStore) Save(botID int64, offset int) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	offsets, err := s.read()
	if err != nil {
		s.warn("Failed to read Telegram cursor file", err)
		offsets = map[string]int{}
	}
	offsets[strconv.FormatInt(botID, 10)] = offset
	return s.write(offsets)
}

// read returns the stored cursors. A missing file is an empty cursor set.
func (s *FileOffsetStore) read() (map[string]int, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]int{}, nil
		}
		return nil, fmt.Errorf("read telegram cursor file: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]int{}, nil
	}
	var payload telegramOffsetFile
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("parse telegram cursor file: %w", err)
	}
	if payload.Offsets == nil {
		payload.Offsets = map[string]int{}
	}
	return payload.Offsets, nil
}

// write replaces the cursor file atomically.
func (s *FileOffsetStore) write(offsets map[string]int) error {
	payload, err := json.Marshal(telegramOffsetFile{Offsets: offsets})
	if err != nil {
		return fmt.Errorf("encode telegram cursor file: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(s.path), ".telegram_offsets-*.tmp")
	if err != nil {
		return fmt.Errorf("create telegram cursor temp file: %w", err)
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return fmt.Errorf("secure telegram cursor temp file: %w", err)
	}
	if _, err := temp.Write(payload); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write telegram cursor temp file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("sync telegram cursor temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close telegram cursor temp file: %w", err)
	}
	if err := os.Rename(tempPath, s.path); err != nil {
		return fmt.Errorf("replace telegram cursor file: %w", err)
	}
	return nil
}

// warn reports a cursor file problem without failing the bot.
func (s *FileOffsetStore) warn(message string, err error) {
	if s.logger == nil {
		return
	}
	s.logger.Warn(message, "error", err)
}
