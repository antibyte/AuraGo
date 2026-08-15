package memory

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const (
	CharacterNoteCategoryValue        = "value"
	CharacterNoteCategoryHabit        = "habit"
	CharacterNoteCategoryRelationship = "relationship"
	CharacterNoteCategorySignature    = "signature"
	CharacterNoteCategoryCommitment   = "commitment"

	CharacterNoteSourceMilestone  = "milestone"
	CharacterNoteSourceReflection = "reflection"
	CharacterNoteSourceUser       = "user_stated"
	CharacterNoteSourceEvent      = "event"

	MaxActiveCharacterNotes       = 20
	MaxCharacterNoteRunes         = 160
	MaxCharacterNotesPerDay       = 2
	MaxCharacterNotesInPrompt     = 6
	CharacterNoteDecayDays        = 30
	characterNoteSettingNarrative = "narrative_visible"
)

// CharacterNote is one durable sentence about the agent itself.
type CharacterNote struct {
	ID         int64   `json:"id"`
	Category   string  `json:"category"`
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	Source     string  `json:"source"`
	Protected  bool    `json:"protected"`
	UpdatedAt  string  `json:"updated_at"`
	CreatedAt  string  `json:"created_at"`
}

const characterNotesSchema = `
CREATE TABLE IF NOT EXISTS character_notes (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	category TEXT NOT NULL,
	text TEXT NOT NULL,
	text_hash TEXT NOT NULL UNIQUE,
	confidence REAL NOT NULL DEFAULT 0.6,
	source TEXT NOT NULL,
	protected INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS character_note_rejections (
	text_hash TEXT PRIMARY KEY,
	rejected_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS personality_settings (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS character_milestone_reviews (
	label TEXT PRIMARY KEY,
	reviewed_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
`

// InitCharacterNoteTables creates the lived-character ledger.
func (s *SQLiteMemory) InitCharacterNoteTables() error {
	if _, err := s.db.Exec(characterNotesSchema); err != nil {
		return fmt.Errorf("character notes schema: %w", err)
	}
	return nil
}

// ListCharacterNotes returns active notes, protected first, then recency×confidence.
func (s *SQLiteMemory) ListCharacterNotes() ([]CharacterNote, error) {
	rows, err := s.db.Query(`
		SELECT id, category, text, confidence, source, protected, created_at, updated_at
		FROM character_notes
		ORDER BY protected DESC, (confidence * (1.0 / (1.0 + (julianday('now') - julianday(updated_at))))) DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list character notes: %w", err)
	}
	defer rows.Close()
	var notes []CharacterNote
	for rows.Next() {
		var n CharacterNote
		var protected int
		if err := rows.Scan(&n.ID, &n.Category, &n.Text, &n.Confidence, &n.Source, &protected, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan character note: %w", err)
		}
		n.Protected = protected == 1
		notes = append(notes, n)
	}
	return notes, rows.Err()
}

// InsertCharacterNote stores a clamped note unless it was rejected or is a duplicate.
func (s *SQLiteMemory) InsertCharacterNote(note CharacterNote) (CharacterNote, error) {
	note = NormalizeCharacterNote(note)
	if note.Text == "" {
		return CharacterNote{}, fmt.Errorf("character note text is empty")
	}
	hash := characterNoteHash(note.Text)
	var rejected int
	if err := s.db.QueryRow(`SELECT COUNT(1) FROM character_note_rejections WHERE text_hash = ?`, hash).Scan(&rejected); err != nil {
		return CharacterNote{}, fmt.Errorf("check character note rejection: %w", err)
	}
	if rejected > 0 {
		return CharacterNote{}, fmt.Errorf("character note was deleted by the user")
	}
	res, err := s.db.Exec(
		`INSERT INTO character_notes (category, text, text_hash, confidence, source, protected)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(text_hash) DO UPDATE SET
			confidence = MAX(character_notes.confidence, excluded.confidence),
			updated_at = CURRENT_TIMESTAMP`,
		note.Category, note.Text, hash, note.Confidence, note.Source, characterNoteBoolToInt(note.Protected),
	)
	if err != nil {
		return CharacterNote{}, fmt.Errorf("insert character note: %w", err)
	}
	_ = res
	var protected int
	if err := s.db.QueryRow(
		`SELECT id, category, text, confidence, source, protected, created_at, updated_at
		 FROM character_notes WHERE text_hash = ?`, hash,
	).Scan(&note.ID, &note.Category, &note.Text, &note.Confidence, &note.Source, &protected, &note.CreatedAt, &note.UpdatedAt); err != nil {
		return CharacterNote{}, fmt.Errorf("reload character note: %w", err)
	}
	note.Protected = protected == 1
	return note, s.enforceCharacterNoteCap()
}

// DeleteCharacterNote removes a note and remembers its hash so reflection cannot restore it.
func (s *SQLiteMemory) DeleteCharacterNote(id int64) error {
	var hash string
	err := s.db.QueryRow(`SELECT text_hash FROM character_notes WHERE id = ?`, id).Scan(&hash)
	if err == sql.ErrNoRows {
		return fmt.Errorf("character note %d not found", id)
	}
	if err != nil {
		return fmt.Errorf("lookup character note: %w", err)
	}
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO character_note_rejections (text_hash) VALUES (?)`, hash); err != nil {
		return fmt.Errorf("remember rejected character note: %w", err)
	}
	if _, err := s.db.Exec(`DELETE FROM character_notes WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete character note: %w", err)
	}
	return nil
}

// SetCharacterNoteProtected pins or unpins a note against decay.
func (s *SQLiteMemory) SetCharacterNoteProtected(id int64, protected bool) error {
	res, err := s.db.Exec(`UPDATE character_notes SET protected = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, characterNoteBoolToInt(protected), id)
	if err != nil {
		return fmt.Errorf("protect character note: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("character note %d not found", id)
	}
	return nil
}

func (s *SQLiteMemory) CountAllCharacterNotes() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(1) FROM character_notes`).Scan(&n)
	return n, err
}

// CountCharacterNotesCreatedSince counts inserts/updates from the given time.
func (s *SQLiteMemory) CountCharacterNotesCreatedSince(since time.Time) (int, error) {
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(1) FROM character_notes WHERE created_at >= ?`,
		since.UTC().Format("2006-01-02 15:04:05"),
	).Scan(&n)
	return n, err
}

// DecayUnprotectedCharacterNotes removes stale, unpinned, low-confidence notes.
func (s *SQLiteMemory) DecayUnprotectedCharacterNotes(now time.Time) (int, error) {
	if now.IsZero() {
		now = time.Now()
	}
	cutoff := now.Add(-time.Duration(CharacterNoteDecayDays) * 24 * time.Hour).UTC().Format("2006-01-02 15:04:05")
	res, err := s.db.Exec(
		`DELETE FROM character_notes
		 WHERE protected = 0 AND confidence < 0.75 AND updated_at < ?`,
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("decay character notes: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// FormatCharacterNotesForPrompt returns the top trusted identity sentences.
func (s *SQLiteMemory) FormatCharacterNotesForPrompt(limit int) string {
	if limit <= 0 || limit > MaxCharacterNotesInPrompt {
		limit = MaxCharacterNotesInPrompt
	}
	notes, err := s.ListCharacterNotes()
	if err != nil || len(notes) == 0 {
		return ""
	}
	if len(notes) > limit {
		notes = notes[:limit]
	}
	var b strings.Builder
	for _, note := range notes {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("- ")
		b.WriteString(note.Text)
	}
	return b.String()
}

// NarrativeVisible reports whether the agent may subtly disclose lived personality.
func (s *SQLiteMemory) NarrativeVisible() bool {
	var value string
	if err := s.db.QueryRow(`SELECT value FROM personality_settings WHERE key = ?`, characterNoteSettingNarrative).Scan(&value); err != nil {
		return false
	}
	return value == "1" || strings.EqualFold(value, "true")
}

// SetNarrativeVisible stores the dashboard narrative toggle.
func (s *SQLiteMemory) SetNarrativeVisible(visible bool) error {
	_, err := s.db.Exec(
		`INSERT INTO personality_settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		characterNoteSettingNarrative, map[bool]string{true: "1", false: "0"}[visible],
	)
	return err
}

// UnreviewedMilestones returns milestone labels that still need a dashboard review.
func (s *SQLiteMemory) UnreviewedMilestones() ([]string, error) {
	rows, err := s.db.Query(`
		SELECT m.label FROM character_milestones m
		LEFT JOIN character_milestone_reviews r ON r.label = m.label
		WHERE r.label IS NULL
		ORDER BY m.id DESC LIMIT 8`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var labels []string
	for rows.Next() {
		var label string
		if err := rows.Scan(&label); err != nil {
			return nil, err
		}
		labels = append(labels, label)
	}
	return labels, rows.Err()
}

// MarkMilestoneReviewed hides a milestone from the dashboard review badge.
func (s *SQLiteMemory) MarkMilestoneReviewed(label string) error {
	label = strings.TrimSpace(label)
	if label == "" {
		return fmt.Errorf("milestone label is empty")
	}
	_, err := s.db.Exec(
		`INSERT INTO character_milestone_reviews (label) VALUES (?)
		 ON CONFLICT(label) DO UPDATE SET reviewed_at = CURRENT_TIMESTAMP`,
		label,
	)
	return err
}

func (s *SQLiteMemory) enforceCharacterNoteCap() error {
	notes, err := s.ListCharacterNotes()
	if err != nil || len(notes) <= MaxActiveCharacterNotes {
		return err
	}
	for i := MaxActiveCharacterNotes; i < len(notes); i++ {
		if notes[i].Protected {
			continue
		}
		if _, err := s.db.Exec(`DELETE FROM character_notes WHERE id = ?`, notes[i].ID); err != nil {
			return err
		}
	}
	return nil
}

func characterNoteHash(text string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.Join(strings.Fields(text), " "))))
	return hex.EncodeToString(sum[:])
}

func characterNoteBoolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func validCharacterNoteCategory(v string) bool {
	switch v {
	case CharacterNoteCategoryValue, CharacterNoteCategoryHabit, CharacterNoteCategoryRelationship, CharacterNoteCategorySignature, CharacterNoteCategoryCommitment:
		return true
	default:
		return false
	}
}

func validCharacterNoteSource(v string) bool {
	switch v {
	case CharacterNoteSourceMilestone, CharacterNoteSourceReflection, CharacterNoteSourceUser, CharacterNoteSourceEvent:
		return true
	default:
		return false
	}
}
