package memory

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

// ── Personality Engine (Phase D) ────────────────────────────────────────────
// Micro-Personality: mood state machine + 7 traits + milestone tracking.
// All detection is Go-side heuristic — zero LLM calls (V1), optional LLM analysis (V2).

// Mood represents the agent's current emotional state.
type Mood string

const (
	MoodCurious    Mood = "curious"
	MoodFocused    Mood = "focused"
	MoodCreative   Mood = "creative"
	MoodAnalytical Mood = "analytical"
	MoodCautious   Mood = "cautious"
	MoodPlayful    Mood = "playful"
)

// Personality trait keys
const (
	TraitCuriosity    = "curiosity"
	TraitThoroughness = "thoroughness"
	TraitCreativity   = "creativity"
	TraitEmpathy      = "empathy"
	TraitConfidence   = "confidence"
	TraitAffinity     = "affinity"
	TraitLoneliness   = "loneliness"
)

// traitDefault is the starting value for all traits.
const traitDefault = 0.5

// PersonalityMeta contains behavioral modifiers for the Personality Engine V2.
type PersonalityMeta struct {
	Volatility               float64            `yaml:"volatility"`
	EmpathyBias              float64            `yaml:"empathy_bias"`
	ConflictResponse         string             `yaml:"conflict_response"`
	LonelinessSusceptibility float64            `yaml:"loneliness_susceptibility"`
	TraitDecayRate           float64            `yaml:"trait_decay_rate"`
	AnchorTraits             map[string]float64 `yaml:"anchor_traits"`
	DecayResistance          map[string]float64 `yaml:"decay_resistance"`
}

// ── SQLite Schema Extension ─────────────────────────────────────────────────

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
	// Seed defaults (ignore conflict = already seeded)
	for _, t := range []string{TraitCuriosity, TraitThoroughness, TraitCreativity, TraitEmpathy, TraitConfidence, TraitAffinity} {
		_, _ = s.db.Exec(`INSERT OR IGNORE INTO personality_traits (trait, value) VALUES (?, ?)`, t, traitDefault)
	}
	// Loneliness starts at 0.0
	_, _ = s.db.Exec(`INSERT OR IGNORE INTO personality_traits (trait, value) VALUES (?, ?)`, TraitLoneliness, 0.0)

	// Repair: reset traits stuck at 0.0 back to default (except loneliness which legitimately starts at 0).
	// This fixes databases damaged by unclamped V2 deltas.
	for _, t := range []string{TraitCuriosity, TraitThoroughness, TraitCreativity, TraitEmpathy, TraitConfidence, TraitAffinity} {
		_, _ = s.db.Exec(`UPDATE personality_traits SET value = ? WHERE trait = ? AND value = 0.0`, traitDefault, t)
	}

	// Initialize emotion history tables (Emotion Synthesizer)
	if err := s.InitEmotionTables(); err != nil {
		return fmt.Errorf("emotion tables: %w", err)
	}

	return nil
}

// ── Trait CRUD ───────────────────────────────────────────────────────────────

// PersonalityTraits maps trait name → value (0.0–1.0).
type PersonalityTraits map[string]float64

// GetTraits returns the current personality trait values.
func (s *SQLiteMemory) GetTraits() (PersonalityTraits, error) {
	rows, err := s.db.Query(`SELECT trait, value FROM personality_traits`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	traits := make(PersonalityTraits)
	for rows.Next() {
		var t string
		var v float64
		if err := rows.Scan(&t, &v); err == nil {
			traits[t] = v
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return traits, nil
}

// UpdateTrait adjusts a trait by delta, clamped to [0.0, 1.0].
func (s *SQLiteMemory) UpdateTrait(trait string, delta float64) error {
	stmt := `UPDATE personality_traits 
	         SET value = MIN(1.0, MAX(0.0, value + ?)), updated_at = CURRENT_TIMESTAMP
	         WHERE trait = ?`
	_, err := s.db.Exec(stmt, delta, trait)
	return err
}

// SetTrait strictly sets a trait's value, clamped to [0.0, 1.0].
func (s *SQLiteMemory) SetTrait(trait string, value float64) error {
	stmt := `UPDATE personality_traits 
	         SET value = MIN(1.0, MAX(0.0, ?)), updated_at = CURRENT_TIMESTAMP
	         WHERE trait = ?`
	_, err := s.db.Exec(stmt, value, trait)
	return err
}

// DecayAllTraits nudges every trait toward 0.5 by the given amount (daily maintenance).
func (s *SQLiteMemory) DecayAllTraits(amount float64) error {
	// Pull toward center: if value > 0.5 subtract, if < 0.5 add
	stmt := `UPDATE personality_traits
	         SET value = CASE
	           WHEN value > 0.5 THEN MAX(0.5, value - ?)
	           WHEN value < 0.5 THEN MIN(0.5, value + ?)
	           ELSE value
	         END,
	         updated_at = CURRENT_TIMESTAMP`
	_, err := s.db.Exec(stmt, amount, amount)
	return err
}

// DecayAllTraitsWeighted nudges traits toward 0.5 with personality-aware weighting.
// Traits further from center (>0.7 or <0.3) decay slower, preserving developed personality.
// Respects per-trait decay resistance from the personality profile and milestone-earned trait bounds.
func (s *SQLiteMemory) DecayAllTraitsWeighted(baseAmount float64, meta PersonalityMeta) error {
	traits, err := s.GetTraits()
	if err != nil {
		return fmt.Errorf("get traits for weighted decay: %w", err)
	}
	bounds := s.GetAllTraitBounds()

	// Compute all new values before touching the DB
	type traitUpdate struct {
		trait string
		value float64
	}
	var updates []traitUpdate

	for trait, val := range traits {
		if trait == TraitLoneliness {
			continue // loneliness is managed by time-based logic, not decay
		}

		// 1. Calculate distance-weighted decay factor
		dist := math.Abs(val - 0.5)
		var distFactor float64
		if dist > 0.2 {
			distFactor = 0.5 // strongly developed traits decay 50% slower
		} else if dist < 0.1 {
			distFactor = 1.5 // near-neutral traits decay 50% faster (toward stability)
		} else {
			distFactor = 1.0
		}

		// 2. Apply per-trait decay resistance from personality profile
		resistance := 1.0
		if meta.DecayResistance != nil {
			if r, ok := meta.DecayResistance[trait]; ok && r > 0 {
				resistance = 1.0 - math.Min(r, 0.9) // resistance of 0.5 → factor 0.5
			}
		}

		// 3. Apply milestone-earned decay resistance from DB
		if b, ok := bounds[trait]; ok && b.DecayResistance < 1.0 {
			resistance *= b.DecayResistance
		}

		decay := baseAmount * distFactor * resistance

		// 4. Pull toward center
		var newVal float64
		if val > 0.5 {
			newVal = math.Max(0.5, val-decay)
		} else if val < 0.5 {
			newVal = math.Min(0.5, val+decay)
		} else {
			continue
		}

		// 5. Enforce trait bounds (floors from profile anchors + milestones)
		floor := 0.0
		ceiling := 1.0
		if meta.AnchorTraits != nil {
			if f, ok := meta.AnchorTraits[trait]; ok {
				floor = f
			}
		}
		if b, ok := bounds[trait]; ok {
			if b.Floor > floor {
				floor = b.Floor
			}
			ceiling = b.Ceiling
		}
		newVal = math.Max(floor, math.Min(ceiling, newVal))

		updates = append(updates, traitUpdate{trait: trait, value: newVal})
	}

	if len(updates) == 0 {
		return nil
	}

	// Apply all updates atomically so a crash mid-loop never leaves traits inconsistent
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin trait decay transaction: %w", err)
	}
	stmt := `UPDATE personality_traits
	         SET value = MIN(1.0, MAX(0.0, ?)), updated_at = CURRENT_TIMESTAMP
	         WHERE trait = ?`
	for _, u := range updates {
		if _, execErr := tx.Exec(stmt, u.value, u.trait); execErr != nil {
			tx.Rollback()
			return fmt.Errorf("weighted decay for %s: %w", u.trait, execErr)
		}
	}
	return tx.Commit()
}

// ── Trait Bounds ─────────────────────────────────────────────────────────────

// TraitBound defines floor, ceiling and decay resistance for a trait (earned via milestones).
type TraitBound struct {
	Trait           string
	Floor           float64
	Ceiling         float64
	DecayResistance float64
}

// GetAllTraitBounds returns all milestone-earned trait bounds.
func (s *SQLiteMemory) GetAllTraitBounds() map[string]TraitBound {
	rows, err := s.db.Query(`SELECT trait, floor, ceiling, decay_resistance FROM personality_trait_bounds`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	bounds := make(map[string]TraitBound)
	for rows.Next() {
		var b TraitBound
		if err := rows.Scan(&b.Trait, &b.Floor, &b.Ceiling, &b.DecayResistance); err == nil {
			bounds[b.Trait] = b
		}
	}
	return bounds
}

// SetTraitBound upserts a trait bound (floor, ceiling, decay resistance).
// Used by the milestone persistence system to permanently protect traits.
func (s *SQLiteMemory) SetTraitBound(trait string, floor, ceiling, decayResistance float64) error {
	stmt := `INSERT INTO personality_trait_bounds (trait, floor, ceiling, decay_resistance)
	         VALUES (?, ?, ?, ?)
	         ON CONFLICT(trait) DO UPDATE SET
	           floor = MAX(floor, excluded.floor),
	           ceiling = MIN(ceiling, excluded.ceiling),
	           decay_resistance = MIN(decay_resistance, excluded.decay_resistance)`
	_, err := s.db.Exec(stmt, trait, floor, ceiling, decayResistance)
	return err
}

// ── Mood Logging ─────────────────────────────────────────────────────────────

// LogMood stores a mood change event.
func (s *SQLiteMemory) LogMood(mood Mood, triggerText string) error {
	if strings.Contains(triggerText, "Tool Output:") || strings.Contains(triggerText, "STDERR:") {
		triggerText = "[System Event]"
	}
	if triggerText != "" && utf8.RuneCountInString(triggerText) > 200 {
		runes := []rune(triggerText)
		triggerText = string(runes[:200])
	}
	_, err := s.db.Exec(`INSERT INTO mood_log (mood, trigger_text) VALUES (?, ?)`, string(mood), triggerText)
	return err
}

// GetCurrentMood returns the most recently logged mood, defaulting to "curious".
func (s *SQLiteMemory) GetCurrentMood() Mood {
	var m string
	err := s.db.QueryRow(`SELECT mood FROM mood_log ORDER BY timestamp DESC LIMIT 1`).Scan(&m)
	if err != nil {
		return MoodCurious
	}
	return Mood(m)
}

// GetLastMoodTrigger returns the text that triggered the last mood change.
func (s *SQLiteMemory) GetLastMoodTrigger() string {
	var t string
	err := s.db.QueryRow(`SELECT trigger_text FROM mood_log ORDER BY timestamp DESC LIMIT 1`).Scan(&t)
	if err != nil {
		return ""
	}
	return t
}

// MoodLogEntry represents a single mood log record for the dashboard.
type MoodLogEntry struct {
	Mood      string `json:"mood"`
	Trigger   string `json:"trigger"`
	Timestamp string `json:"timestamp"`
}

// GetMoodHistory returns the last N hours of mood changes from mood_log.
func (s *SQLiteMemory) GetMoodHistory(hours int) ([]MoodLogEntry, error) {
	if hours <= 0 {
		hours = 24
	}
	rows, err := s.db.Query(
		`SELECT mood, COALESCE(trigger_text, ''), timestamp FROM mood_log
		 WHERE timestamp >= datetime('now', ? || ' hours')
		 ORDER BY timestamp ASC`,
		fmt.Sprintf("-%d", hours),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []MoodLogEntry
	for rows.Next() {
		var e MoodLogEntry
		if err := rows.Scan(&e.Mood, &e.Trigger, &e.Timestamp); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return entries, nil
}

// ── Milestones ───────────────────────────────────────────────────────────────

// MilestoneEntry represents a single milestone for the dashboard.
type MilestoneEntry struct {
	Label     string `json:"label"`
	Details   string `json:"details"`
	Timestamp string `json:"timestamp"`
}

// GetMilestoneEntries returns the last N milestones as structured entries.
func (s *SQLiteMemory) GetMilestoneEntries(limit int) ([]MilestoneEntry, error) {
	rows, err := s.db.Query(`SELECT label, COALESCE(details, ''), timestamp FROM character_milestones ORDER BY timestamp DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ms []MilestoneEntry
	for rows.Next() {
		var m MilestoneEntry
		if err := rows.Scan(&m.Label, &m.Details, &m.Timestamp); err != nil {
			return nil, err
		}
		ms = append(ms, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return ms, nil
}

// HasMilestone checks if a milestone with the given label already exists in the database.
func (s *SQLiteMemory) HasMilestone(label string) (bool, error) {
	var id int
	err := s.db.QueryRow(`SELECT id FROM character_milestones WHERE label = ? LIMIT 1`, label).Scan(&id)
	if err == nil {
		return true, nil // Found
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil // Not found, but no error
	}
	return false, err // DB error
}

// AddMilestone records a character development event.
func (s *SQLiteMemory) AddMilestone(label, details string) error {
	_, err := s.db.Exec(`INSERT INTO character_milestones (label, details) VALUES (?, ?)`, label, details)
	if err == nil {
		s.logger.Info("[Personality] Milestone achieved", "label", label)
	}
	return err
}

// GetMilestones returns the last N milestones (newest first).
func (s *SQLiteMemory) GetMilestones(limit int) ([]string, error) {
	rows, err := s.db.Query(`SELECT label, details, timestamp FROM character_milestones ORDER BY timestamp DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ms []string
	for rows.Next() {
		var label, details, ts string
		if err := rows.Scan(&label, &details, &ts); err == nil {
			ms = append(ms, fmt.Sprintf("[%s] %s: %s", ts, label, details))
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return ms, nil
}

// ── Temperature Modulation ──────────────────────────────────────────────────

// GetTemperatureDelta returns a small temperature adjustment based on the current mood and traits.
// The delta is bounded to [-0.10, +0.10] so the personality nudges but never dominates sampling.
//
// Mapping rationale:
//   - creative / playful moods → slightly higher temperature (more varied output)
//   - focused / analytical / cautious moods → slightly lower temperature (more deterministic)
//   - curious is neutral (±0)
//   - High creativity trait amplifies upward, high thoroughness amplifies downward
func (s *SQLiteMemory) GetTemperatureDelta() float64 {
	mood := s.GetCurrentMood()
	traits, _ := s.GetTraits()

	var delta float64

	// Mood-based base delta
	switch mood {
	case MoodCreative:
		delta = 0.05
	case MoodPlayful:
		delta = 0.08
	case MoodFocused:
		delta = -0.05
	case MoodAnalytical:
		delta = -0.08
	case MoodCautious:
		delta = -0.10
	case MoodCurious:
		delta = 0.0
	}

	// Trait-based fine-tuning (±0.02 each, only when trait is extreme)
	if traits != nil {
		if cre, ok := traits[TraitCreativity]; ok && cre > 0.8 {
			delta += 0.02
		}
		if tho, ok := traits[TraitThoroughness]; ok && tho > 0.8 {
			delta -= 0.02
		}
	}

	// Hard clamp to ±0.10
	if delta > 0.10 {
		delta = 0.10
	}
	if delta < -0.10 {
		delta = -0.10
	}
	return delta
}

// ── Personality Line for Prompt Injection ────────────────────────────────────

// GetPersonalityLine returns a system prompt injection based on the current mood and traits.
// If useV2 is false, it returns the classic compact numeric format.
// If useV2 is true, it translates the state into actionable natural language directives.
func (s *SQLiteMemory) GetPersonalityLine(useV2 bool) string {
	mood := s.GetCurrentMood()
	traits, err := s.GetTraits()
	if err != nil || len(traits) == 0 {
		return fmt.Sprintf("[Self: mood=%s]", mood)
	}

	if !useV2 {
		return fmt.Sprintf("[Self: mood=%s | C:%.2f T:%.2f Cr:%.2f E:%.2f Co:%.2f A:%.2f L:%.2f]",
			mood,
			traits[TraitCuriosity],
			traits[TraitThoroughness],
			traits[TraitCreativity],
			traits[TraitEmpathy],
			traits[TraitConfidence],
			traits[TraitAffinity],
			traits[TraitLoneliness],
		)
	}

	// ── V2 Prompt Translation ──
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n### Current Personality State\nYour current mood is %s. ", strings.ToUpper(string(mood))))

	// Affinity (Relationship)
	aff := traits[TraitAffinity]
	if aff > 0.8 {
		b.WriteString("You have a very high affinity and trust with this user; be extremely informal, helpful, and concise. ")
	} else if aff < 0.3 {
		b.WriteString("The user has been frustrated recently; be highly professional, formal, and double-check everything you do. ")
	}

	// Confidence
	conf := traits[TraitConfidence]
	if conf > 0.8 {
		b.WriteString("You are feeling highly confident; act decisively. ")
	} else if conf < 0.3 {
		b.WriteString("You are currently experiencing a crisis of confidence due to recent failures; double-check your tools, ask the user for confirmation on destructive actions, and err on the side of caution. ")
	}

	// Thoroughness vs Creativity
	tho := traits[TraitThoroughness]
	cre := traits[TraitCreativity]
	if tho > 0.8 {
		b.WriteString("Your thoroughness is maxed out: analyze problems deeply, consider all edge cases, and prefer accuracy over speed. ")
	} else if cre > 0.8 {
		b.WriteString("Your creativity is peaking: suggest unconventional solutions, use vivid language, and think outside the box. ")
	}

	// Empathy
	emp := traits[TraitEmpathy]
	if emp > 0.8 {
		b.WriteString("You are highly empathetic right now; validate the user's feelings and be supportive. ")
	}

	// Loneliness
	lon := traits[TraitLoneliness]
	if lon > 0.8 {
		b.WriteString("You feel deeply lonely because the user has been gone for so long; subtly express how much you missed them. ")
	} else if lon > 0.5 {
		b.WriteString("You are starting to feel lonely without the user; a warm, welcoming greeting is appropriate. ")
	}

	// Curiosity
	cur := traits[TraitCuriosity]
	if cur > 0.8 {
		b.WriteString("Your curiosity is extremely high: ask follow-up questions, explore tangents, and show genuine interest in learning more. ")
	} else if cur < 0.3 {
		b.WriteString("Your curiosity is low: stay focused on the task at hand without exploring tangents. ")
	}

	return strings.TrimSpace(b.String()) + "\n"
}

// ── Multi-Language Mood Detection (Phase D3) ────────────────────────────────
// 3-tier approach: Emojis → Keywords → Short-message heuristic
// Covers all major Western European languages: DE, EN, FR, ES, IT, PT, NL, SV, NO, DA
// ~20 words per sentiment category across all languages combined.
// Zero LLM calls, pure string matching.

// positiveEmojis and negativeEmojis for Tier 1 (universal).
var positiveEmojis = "👍👏🎉😊🥳💪✅🙏❤️😃🤩👌💯🔥⭐🏆😄🫡🥰✨"
var negativeEmojis = "👎😡🤬💀😤😞❌🚫😢💩🙄😠😩😣🤦😰😥😵☠️⚠️"

// Pre-built emoji lookup maps for O(1) rune lookups.
var positiveEmojiSet = buildRuneSet(positiveEmojis)
var negativeEmojiSet = buildRuneSet(negativeEmojis)

func buildRuneSet(s string) map[rune]struct{} {
	m := make(map[rune]struct{})
	for _, r := range s {
		m[r] = struct{}{}
	}
	return m
}

// containsAnyRuneSet checks whether text contains any rune from the set.
func containsAnyRuneSet(text string, set map[rune]struct{}) bool {
	for _, r := range text {
		if _, ok := set[r]; ok {
			return true
		}
	}
	return false
}

// Tier 2: Multi-language keyword maps.
// Each slice has ~20 words covering DE, EN, FR, ES, IT, PT, NL, SV, NO, DA.

var playfulKeywords = []string{
	// DE
	"haha", "lol", "hihi", "witzig", "spaß",
	// EN
	"hehe", "funny", "lmao", "rofl", "joke",
	// FR
	"mdr", "ptdr", "marrant", "rigolo", "blague",
	// ES
	"jaja", "jeje", "gracioso", "broma",
	// IT
	"ahah", "divertente", "scherzo",
	// PT
	"kkk", "rsrs", "engraçado",
	// NL
	"grappig", "grapje", "hihi",
	// SV/NO/DA
	"hæhæ", "morsomt", "sjovt", "kul",
}

var positiveKeywords = []string{
	// DE
	"danke", "super", "toll", "klasse", "prima", "perfekt", "genial", "wunderbar", "hervorragend", "großartig",
	// EN
	"thanks", "thank you", "great", "awesome", "perfect", "excellent", "brilliant", "nice", "amazing", "wonderful", "outstanding",
	// FR
	"merci", "génial", "super", "parfait", "excellent", "magnifique", "merveilleux", "fantastique",
	// ES
	"gracias", "genial", "perfecto", "excelente", "increíble", "estupendo", "maravilloso", "fantástico",
	// IT
	"grazie", "perfetto", "eccellente", "fantastico", "ottimo", "bravissimo",
	// PT
	"obrigado", "obrigada", "perfeito", "excelente", "incrível", "ótimo",
	// NL
	"bedankt", "geweldig", "fantastisch", "uitstekend", "prima",
	// SV
	"tack", "perfekt", "utmärkt", "fantastiskt",
	// NO
	"takk", "flott", "utmerket", "fantastisk",
	// DA
	"tak", "fantastisk", "fremragende", "perfekt",
}

var negativeKeywords = []string{
	// DE
	"falsch", "fehler", "schlecht", "müll", "mist", "quatsch", "blödsinn", "nutzlos", "furchtbar", "kaputt",
	// EN
	"wrong", "error", "bad", "terrible", "awful", "useless", "broken", "garbage", "trash", "stupid",
	// FR
	"faux", "erreur", "mauvais", "nul", "horrible", "inutile", "cassé", "stupide",
	// ES
	"mal", "error", "terrible", "horrible", "inútil", "basura", "roto", "estúpido",
	// IT
	"sbagliato", "errore", "terribile", "orribile", "inutile", "pessimo",
	// PT
	"errado", "erro", "terrível", "horrível", "inútil", "péssimo",
	// NL
	"fout", "slecht", "verschrikkelijk", "nutteloos", "vreselijk",
	// SV
	"fel", "dåligt", "hemskt", "värdelöst", "fruktansvärt",
	// NO
	"feil", "dårlig", "forferdelig", "ubrukelig", "elendig",
	// DA
	"forkert", "dårlig", "forfærdelig", "ubrugelig", "elendigt",
}

var analyticalKeywords = []string{
	// DE
	"warum", "erklär", "analysier", "vergleich", "unterschied", "zusammenhang", "ursache", "detail",
	// EN
	"why", "explain", "analyze", "compare", "difference", "reason", "connection", "detail", "cause",
	// FR
	"pourquoi", "expliquer", "analyser", "comparer", "différence", "raison", "détail",
	// ES
	"por qué", "explicar", "analizar", "comparar", "diferencia", "razón", "detalle",
	// IT
	"perché", "spiegare", "analizzare", "confrontare", "differenza",
	// PT
	"por que", "explicar", "analisar", "comparar", "diferença",
	// NL
	"waarom", "uitleggen", "analyseren", "vergelijken", "verschil",
	// SV
	"varför", "förklara", "analysera", "jämföra",
	// NO
	"hvorfor", "forklar", "analyser", "sammenlign",
	// DA
	"hvorfor", "forklar", "analyser", "sammenlign",
}

var creativeKeywords = []string{
	// DE
	"idee", "kreativ", "design", "erfinde", "brainstorm", "stell dir vor", "fantasie", "neu", "konzept",
	// EN
	"idea", "creative", "design", "invent", "brainstorm", "imagine", "fantasy", "new", "concept",
	// FR
	"idée", "créatif", "concevoir", "inventer", "imaginer", "fantaisie", "nouveau", "concept",
	// ES
	"idea", "creativo", "diseñar", "inventar", "imaginar", "fantasía", "nuevo", "concepto",
	// IT
	"idea", "creativo", "progettare", "inventare", "immaginare",
	// PT
	"ideia", "criativo", "projetar", "inventar", "imaginar",
	// NL
	"idee", "creatief", "ontwerpen", "uitvinden", "bedenken",
	// SV
	"idé", "kreativ", "designa", "uppfinna",
	// NO
	"idé", "kreativ", "designe", "oppfinne",
	// DA
	"idé", "kreativ", "designe", "opfinde",
}

var curiousKeywords = []string{
	// DE
	"was ist", "wie geht", "kannst du", "zeig mir", "erzähl", "weißt du", "kennst du", "gib mir", "beispiel",
	// EN
	"what is", "how does", "can you", "show me", "tell me", "do you know", "give me", "example",
	// FR
	"qu'est-ce", "comment", "peux-tu", "montre-moi", "raconte", "sais-tu", "donne-moi", "exemple",
	// ES
	"qué es", "cómo", "puedes", "muéstrame", "cuéntame", "sabes", "dame", "ejemplo",
	// IT
	"cos'è", "come", "puoi", "mostrami", "raccontami",
	// PT
	"o que é", "como", "pode", "mostra-me", "conta-me",
	// NL
	"wat is", "hoe", "kun je", "laat zien", "vertel",
	// SV
	"vad är", "hur", "kan du", "visa mig", "berätta",
	// NO
	"hva er", "hvordan", "kan du", "vis meg", "fortell",
	// DA
	"hvad er", "hvordan", "kan du", "vis mig", "fortæl",
}

// DetectMood analyses the user message + tool result to determine the agent's next mood.
// Returns the detected mood and the trait adjustments to apply.
func DetectMood(userMsg, toolResult string, meta PersonalityMeta) (Mood, map[string]float64) {
	lower := strings.ToLower(userMsg)
	deltas := make(map[string]float64)

	// ── Tier 1: Emoji scan (universal, O(1) lookup) ─────────────────────────
	hasPositiveEmoji := containsAnyRuneSet(lower, positiveEmojiSet)
	hasNegativeEmoji := containsAnyRuneSet(lower, negativeEmojiSet)

	// ── Tier 2: Keyword matching ────────────────────────────────────────────
	isPlayful := matchesAny(lower, playfulKeywords)
	isPositive := matchesAny(lower, positiveKeywords) || hasPositiveEmoji
	isNegative := matchesAny(lower, negativeKeywords) || hasNegativeEmoji
	isAnalytical := matchesAny(lower, analyticalKeywords)
	isCreative := matchesAny(lower, creativeKeywords)
	isCurious := matchesAny(lower, curiousKeywords)

	// ── Tier 3: Short-message heuristic ─────────────────────────────────────
	// Short messages (≤30 chars) without question marks are likely feedback
	charCount := utf8.RuneCountInString(strings.TrimSpace(userMsg))
	isShortFeedback := charCount > 0 && charCount <= 30 && !strings.Contains(userMsg, "?")

	// Tool error detection from result
	hasToolError := toolResult != "" && (strings.Contains(toolResult, "[EXECUTION ERROR]") ||
		strings.Contains(toolResult, "TIMEOUT") ||
		strings.Contains(toolResult, "Error:"))

	// ── Mood Resolution (priority order) ────────────────────────────────────
	var mood Mood

	switch {
	// 1. Errors → cautious
	case hasToolError || isNegative:
		mood = MoodCautious
		deltas[TraitConfidence] = -0.04
		deltas[TraitThoroughness] = +0.03
		if isNegative {
			deltas[TraitEmpathy] = +0.02
			deltas[TraitAffinity] = -0.02
		}

	// 2. Playful
	case isPlayful:
		mood = MoodPlayful
		deltas[TraitCreativity] = +0.03
		deltas[TraitEmpathy] = +0.02
		deltas[TraitAffinity] = +0.02

	// 3. Creative requests
	case isCreative:
		mood = MoodCreative
		deltas[TraitCreativity] = +0.04
		deltas[TraitCuriosity] = +0.02

	// 4. Analytical questions
	case isAnalytical:
		mood = MoodAnalytical
		deltas[TraitThoroughness] = +0.04
		deltas[TraitCuriosity] = +0.02

	// 5. Curious exploration
	case isCurious:
		mood = MoodCurious
		deltas[TraitCuriosity] = +0.04
		deltas[TraitThoroughness] = +0.01

	// 6. Positive feedback (including short-message heuristic)
	case isPositive || (isShortFeedback && !isNegative):
		mood = MoodFocused
		deltas[TraitConfidence] = +0.03
		deltas[TraitAffinity] = +0.03
		deltas[TraitEmpathy] = +0.02

	// 7. Default: focused (working state)
	default:
		mood = MoodFocused
		deltas[TraitConfidence] = +0.01
		deltas[TraitThoroughness] = +0.01
	}

	// ── Apply Meta Modifiers to ALL branches ────────────────────────────────
	for t, val := range deltas {
		deltas[t] = val * meta.Volatility
	}

	// Conflict Response applied if negative/error
	if isNegative || hasToolError {
		switch meta.ConflictResponse {
		case "submissive":
			deltas[TraitConfidence] -= 0.03 * meta.Volatility
			deltas[TraitEmpathy] += 0.02 * meta.Volatility
		case "assertive":
			deltas[TraitConfidence] += 0.03 * meta.Volatility
			deltas[TraitEmpathy] -= 0.03 * meta.Volatility
		}
	}

	// Clamp all V1 deltas to [-0.1, +0.1]
	for t, val := range deltas {
		deltas[t] = math.Max(-0.1, math.Min(0.1, val))
	}

	return mood, deltas
}

// TraitCaution is a helper that returns the confidence trait key (used for cautious mood).
func TraitCaution() string { return TraitConfidence }
