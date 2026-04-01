package memory

import "fmt"

// personalitySchema contains the DDL for personality tables.
// Called from InitPersonalityTables.
const personalitySchema = `
CREATE TABLE IF NOT EXISTS personality_traits (
	trait TEXT PRIMARY KEY,
	value REAL DEFAULT 0.5,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS mood_log (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	mood TEXT NOT NULL,
	trigger_text TEXT,
	timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_mood_log_time ON mood_log(timestamp);

CREATE TABLE IF NOT EXISTS character_milestones (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	label TEXT NOT NULL,
	details TEXT,
	timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS personality_trait_bounds (
	trait TEXT PRIMARY KEY,
	floor REAL DEFAULT 0.0,
	ceiling REAL DEFAULT 1.0,
	decay_resistance REAL DEFAULT 1.0
);`

// InitPersonalityTables creates the personality-related tables and seeds default traits.
func (s *SQLiteMemory) InitPersonalityTables() error {
	if _, err := s.db.Exec(personalitySchema); err != nil {
		return fmt.Errorf("personality schema: %w", err)
	}
	for _, trait := range []string{TraitCuriosity, TraitThoroughness, TraitCreativity, TraitEmpathy, TraitConfidence, TraitAffinity} {
		_, _ = s.db.Exec(`INSERT OR IGNORE INTO personality_traits (trait, value) VALUES (?, ?)`, trait, traitDefault)
	}
	_, _ = s.db.Exec(`INSERT OR IGNORE INTO personality_traits (trait, value) VALUES (?, ?)`, TraitLoneliness, 0.0)

	for _, trait := range []string{TraitCuriosity, TraitThoroughness, TraitCreativity, TraitEmpathy, TraitConfidence, TraitAffinity} {
		_, _ = s.db.Exec(`UPDATE personality_traits SET value = ? WHERE trait = ? AND value = 0.0`, traitDefault, trait)
	}

	if err := s.InitEmotionTables(); err != nil {
		return fmt.Errorf("emotion tables: %w", err)
	}

	return nil
}
