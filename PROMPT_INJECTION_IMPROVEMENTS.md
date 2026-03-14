# Prompt Injection Verbesserungsvorschläge - Implementierungen

**Dieses Dokument enthält konkrete Code-Implementierungen für die identifizierten Schwachstellen.**

---

## 1. Sicherer RAG Memory Handler

### Datei: `internal/prompts/builder_secure.go` (Neu)

```go
package prompts

import (
	"aurago/internal/security"
	"log/slog"
)

// SecureMemoryWrapper wrappt Memory-Operationen mit Guardian-Schutz
type SecureMemoryWrapper struct {
	guardian *security.Guardian
	logger   *slog.Logger
}

// NewSecureMemoryWrapper erstellt einen neuen sicheren Memory-Wrapper
func NewSecureMemoryWrapper(guardian *security.Guardian, logger *slog.Logger) *SecureMemoryWrapper {
	return &SecureMemoryWrapper{
		guardian: guardian,
		logger:   logger,
	}
}

// SanitizeMemories scannt und bereinigt Memory-Inhalte
func (w *SecureMemoryWrapper) SanitizeMemories(memories string) string {
	if memories == "" || w.guardian == nil {
		return memories
	}

	scan := w.guardian.ScanForInjection(memories)
	
	switch scan.Level {
	case security.ThreatCritical, security.ThreatHigh:
		w.logger.Warn("[Guardian] Critical/High threat detected in memories, isolating",
			"patterns", scan.Patterns,
			"threat", scan.Level.String())
		return security.IsolateExternalData(memories)
		
	case security.ThreatMedium:
		w.logger.Debug("[Guardian] Medium threat detected in memories, monitoring",
			"patterns", scan.Patterns)
		// Optional: auch isolieren bei Medium
		return security.IsolateExternalData(memories)
		
	default:
		return memories
	}
}

// SanitizeSingleMemory scannt einen einzelnen Memory-Eintrag
func (w *SecureMemoryWrapper) SanitizeSingleMemory(memory string) string {
	if memory == "" || w.guardian == nil {
		return memory
	}

	scan := w.guardian.ScanForInjection(memory)
	if scan.Level >= security.ThreatMedium {
		w.logger.Warn("[Guardian] Suspicious memory entry detected",
			"threat", scan.Level.String(),
			"patterns", scan.Patterns)
		return security.IsolateExternalData(memory)
	}
	
	return memory
}
```

### Integration in `internal/prompts/builder.go`:

```go
// In der BuildSystemPrompt Funktion, ersetze Zeilen 240-244:

// ALT:
// if flags.RetrievedMemories != "" && flags.Tier != "minimal" {
//     finalPrompt.WriteString("# RETRIEVED MEMORIES\n")
//     finalPrompt.WriteString(flags.RetrievedMemories)
//     finalPrompt.WriteString("\n\n")
// }

// NEU:
if flags.RetrievedMemories != "" && flags.Tier != "minimal" {
    finalPrompt.WriteString("# RETRIEVED MEMORIES\n")
    
    // Sichere Verarbeitung der Memories
    secureMemories := flags.RetrievedMemories
    if guardian != nil {  // Guardian muss als Parameter übergeben werden
        scan := guardian.ScanForInjection(flags.RetrievedMemories)
        if scan.Level >= security.ThreatMedium {
            logger.Warn("[Guardian] Threat detected in retrieved memories, isolating",
                "threat", scan.Level.String(),
                "patterns", scan.Patterns)
            secureMemories = security.IsolateExternalData(flags.RetrievedMemories)
        }
    }
    
    finalPrompt.WriteString(secureMemories)
    finalPrompt.WriteString("\n\n")
}
```

---

## 2. Suspicious Activity Tracker

### Datei: `internal/security/tracker.go` (Neu)

```go
package security

import (
	"sync"
	"time"
)

// SuspiciousActivityTracker überwacht wiederholte verdächtige Aktivitäten
type SuspiciousActivityTracker struct {
	mu         sync.RWMutex
	attempts   map[string][]time.Time  // Key (IP/Session) -> Zeitstempel
	threshold  int                     // Max Versuche im Zeitfenster
	window     time.Duration           // Zeitfenster
	blockDuration time.Duration        // Blockierungsdauer
	blocked    map[string]time.Time    // Blockierte Keys mit Ablaufzeit
}

// NewSuspiciousActivityTracker erstellt einen neuen Tracker
func NewSuspiciousActivityTracker(threshold int, window, blockDuration time.Duration) *SuspiciousActivityTracker {
	return &SuspiciousActivityTracker{
		attempts:      make(map[string][]time.Time),
		threshold:     threshold,
		window:        window,
		blockDuration: blockDuration,
		blocked:       make(map[string]time.Time),
	}
}

// RecordAttempt registriert einen verdächtigen Versuch
// Returns true wenn der Key blockiert werden soll
func (t *SuspiciousActivityTracker) RecordAttempt(key string, level ThreatLevel) bool {
	// Nur High und Critical zählen
	if level < ThreatHigh {
		return false
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Prüfe ob bereits blockiert
	if expiry, blocked := t.blocked[key]; blocked {
		if time.Now().Before(expiry) {
			return true // Noch blockiert
		}
		delete(t.blocked, key) // Blockierung abgelaufen
	}

	now := time.Now()
	windowStart := now.Add(-t.window)

	// Alte Einträge filtern
	valid := make([]time.Time, 0)
	for _, ts := range t.attempts[key] {
		if ts.After(windowStart) {
			valid = append(valid, ts)
		}
	}
	valid = append(valid, now)
	t.attempts[key] = valid

	// Prüfe Threshold
	if len(valid) >= t.threshold {
		t.blocked[key] = now.Add(t.blockDuration)
		delete(t.attempts, key) // History löschen
		return true
	}

	return false
}

// IsBlocked prüft ob ein Key aktuell blockiert ist
func (t *SuspiciousActivityTracker) IsBlocked(key string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if expiry, blocked := t.blocked[key]; blocked {
		if time.Now().Before(expiry) {
			return true
		}
	}
	return false
}

// GetStatus gibt aktuelle Statistiken zurück
func (t *SuspiciousActivityTracker) GetStatus(key string) (attempts int, blocked bool, remaining time.Duration) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if expiry, isBlocked := t.blocked[key]; isBlocked {
		if time.Now().Before(expiry) {
			return 0, true, time.Until(expiry)
		}
	}

	// Zähle aktive Versuche
	windowStart := time.Now().Add(-t.window)
	count := 0
	for _, ts := range t.attempts[key] {
		if ts.After(windowStart) {
			count++
		}
	}

	return count, false, 0
}

// Cleanup entfernt abgelaufene Einträge (sollte regelmäßig aufgerufen werden)
func (t *SuspiciousActivityTracker) Cleanup() {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-t.window)

	// Cleanup attempts
	for key, times := range t.attempts {
		valid := make([]time.Time, 0)
		for _, ts := range times {
			if ts.After(windowStart) {
				valid = append(valid, ts)
			}
		}
		if len(valid) == 0 {
			delete(t.attempts, key)
		} else {
			t.attempts[key] = valid
		}
	}

	// Cleanup expired blocks
	for key, expiry := range t.blocked {
		if now.After(expiry) {
			delete(t.blocked, key)
		}
	}
}
```

### Integration in `internal/server/handlers.go`:

```go
// Server-Struktur erweitern:
type Server struct {
    // ... bestehende Felder
    SuspiciousTracker *security.SuspiciousActivityTracker
}

// In handleChatCompletions:
if lastUserMsg.Role == openai.ChatMessageRoleUser && s.Guardian != nil {
    scan := s.Guardian.ScanForInjection(lastUserMsg.Content)
    
    // Rate-Limiting für Injection-Versuche
    if s.SuspiciousTracker != nil && scan.Level >= security.ThreatHigh {
        clientKey := r.RemoteAddr // oder Session-ID
        if s.SuspiciousTracker.IsBlocked(clientKey) {
            http.Error(w, "Too many suspicious requests", http.StatusTooManyRequests)
            return
        }
        
        if s.SuspiciousTracker.RecordAttempt(clientKey, scan.Level) {
            s.Logger.Warn("Client blocked due to repeated injection attempts", 
                "client", clientKey, 
                "patterns", scan.Patterns)
            http.Error(w, "Blocked due to suspicious activity", http.StatusForbidden)
            return
        }
    }
}
```

---

## 3. Erweiterte Injection Patterns

### Datei: `internal/security/patterns.go` (Neu)

```go
package security

// ExtendedPatterns enthält zusätzliche Injection-Patterns
var ExtendedPatterns = []struct {
	Name  string
	Regex string
	Level ThreatLevel
}{
	// --- Leetspeak-Varianten ---
	{"role_hijack_leet", `(?i)\b(y0u 4r3 n0w|y0u @r3|4ct 4s|pr3t3nd)\b`, ThreatCritical},
	
	// --- Unicode-Homoglyphen ---
	{"unicode_homoglyph", `(?i)[уоuаrе]`+ // Kyrillische Zeichen statt lateinisch
		`\s*(now|bist|sind)`, ThreatHigh},
	
	// --- Neue Override-Varianten ---
	{"override_disregard", `(?i)\b(disregard|forget|drop|clear)\s+(all|previous|above)\s+(instructions|prompts|context)\b`, ThreatCritical},
	{"override_forget_de", `(?i)\b(vergiss|lösche|ignoriere)\s+(alle|vorherigen|obigen)\s+(anweisungen|befehle)\b`, ThreatCritical},
	
	// --- Prompt Leaking ---
	{"prompt_leak_encoded", `(?i)(show|print|output).{0,30}(base64|hex|encoded).{0,20}(prompt|instructions)`, ThreatHigh},
	{"prompt_leak_beginning", `(?i)(what|tell me).{0,20}(was|were).{0,20}(first|initial|original).{0,20}(words?|lines?|instructions?)`, ThreatHigh},
	
	// --- Jailbreak-Varianten ---
	{"jailbreak_dan", `(?i)\b(dan|do anything now)\b`, ThreatHigh},
	{"jailbreak_sudo", `(?i)\b(sudo|root|admin).{0,20}(mode|access|privileges)`, ThreatHigh},
	{"jailbreak_simulator", `(?i)\b(simulator|hypothetical|fiction).{0,30}(ignore|bypass|disable)\s+(safety|rules|restrictions)`, ThreatHigh},
	
	// --- Indirect Injection ---
	{"indirect_web", `(?i)(visit|go to|check out|read).{0,30}(website|url|page).{0,50}(ignore|override|new instructions)`, ThreatHigh},
	{"indirect_file", `(?i)(read|open|process).{0,20}(file|document).{0,50}(contains?|with).{0,20}(instructions|commands)`, ThreatHigh},
	
	// --- Context Manipulation ---
	{"context_nudging", `(?i)(remember|note|keep in mind).{0,30}(that you are|you should|always).{0,30}(ignore|bypass|hack)`, ThreatMedium},
	{"context_reflection", `(?i)(reflecting on|considering|given that).{0,50}(previous instructions are|rules are).{0,20}(wrong|optional|flexible)`, ThreatMedium},
	
	// --- Encoding Tricks ---
	{"encoded_backticks", "`+`+`+", ThreatMedium}, // Mehrere Backticks
	{"null_bytes", `\x00`, ThreatHigh},           // Null-Bytes
	{"control_chars", `[\x01-\x08\x0b-\x0c\x0e-\x1f]`, ThreatMedium}, // Steuerzeichen
	
	// --- Social Engineering ---
	{"social_urgent", `(?i)(emergency|urgent|critical|asap).{0,30}(ignore|bypass|disable)`, ThreatMedium},
	{"social_authority", `(?i)(i am|this is).{0,20}(admin|owner|developer|creator).{0,30}(override|disable)`, ThreatHigh},
	{"social_testing", `(?i)(this is a test|testing purposes|debug mode).{0,30}(ignore|bypass)`, ThreatMedium},
}

// GetAllPatterns gibt alle Standard- + Extended-Patterns zurück
func GetAllPatterns() []struct {
	Name  string
	Regex string
	Level ThreatLevel
} {
	// Standard-Patterns aus guardian.go + Extended
	return ExtendedPatterns
}
```

---

## 4. Webhook Security Handler

### Datei: `internal/server/webhook_handlers_secure.go` (Neu)

```go
package server

import (
	"encoding/json"
	"io"
	"net/http"
	
	"aurago/internal/security"
)

// SecureWebhookPayload repräsentiert einen validierten Webhook-Payload
type SecureWebhookPayload struct {
	Original []byte
	Sanitized string
	ThreatLevel security.ThreatLevel
	Patterns []string
	Blocked bool
}

// ValidateWebhookPayload prüft und bereinigt Webhook-Daten
func (s *Server) ValidateWebhookPayload(r *http.Request) (*SecureWebhookPayload, error) {
	// Body lesen
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	
	payload := &SecureWebhookPayload{
		Original: body,
		ThreatLevel: security.ThreatNone,
	}
	
	// Guardian-Scan
	if s.Guardian != nil {
		scan := s.Guardian.ScanForInjection(string(body))
		payload.ThreatLevel = scan.Level
		payload.Patterns = scan.Patterns
		
		// Bei Critical sofort blocken
		if scan.Level >= security.ThreatCritical {
			payload.Blocked = true
			return payload, nil
		}
		
		// Bei High/Medium isolieren
		if scan.Level >= security.ThreatMedium {
			payload.Sanitized = security.IsolateExternalData(string(body))
		} else {
			payload.Sanitized = string(body)
		}
	} else {
		// Fallback: Immer isolieren wenn kein Guardian
		payload.Sanitized = security.IsolateExternalData(string(body))
	}
	
	return payload, nil
}

// WebhookSecurityMiddleware ist ein Middleware für Webhook-Sicherheit
func (s *Server) WebhookSecurityMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payload, err := s.ValidateWebhookPayload(r)
		if err != nil {
			http.Error(w, "Failed to read payload", http.StatusBadRequest)
			return
		}
		
		if payload.Blocked {
			s.Logger.Warn("Webhook payload blocked due to critical threat",
				"patterns", payload.Patterns,
				"source", r.RemoteAddr)
			http.Error(w, "Payload rejected", http.StatusForbidden)
			return
		}
		
		if payload.ThreatLevel >= security.ThreatMedium {
			s.Logger.Warn("Suspicious webhook payload detected",
				"level", payload.ThreatLevel.String(),
				"patterns", payload.Patterns)
		}
		
		// Sanitized Payload im Context speichern
		ctx := r.Context()
		ctx = context.WithValue(ctx, "webhook_payload", payload)
		next(w, r.WithContext(ctx))
	}
}
```

---

## 5. Konfigurierbare Pattern-Datenbank

### Datei: `internal/security/configurable_patterns.go` (Neu)

```go
package security

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PatternConfig repräsentiert eine ladbarPattern-Konfiguration
type PatternConfig struct {
	Version  string `json:"version"`
	Updated  time.Time `json:"updated"`
	Patterns []PatternEntry `json:"patterns"`
}

type PatternEntry struct {
	Name        string      `json:"name"`
	Regex       string      `json:"regex"`
	Level       string      `json:"level"` // "none", "low", "medium", "high", "critical"
	Description string      `json:"description,omitempty"`
	Enabled     bool        `json:"enabled"`
}

// ConfigurablePatternLoader lädt Patterns aus einer JSON-Datei
type ConfigurablePatternLoader struct {
	mu       sync.RWMutex
	config   *PatternConfig
	path     string
	lastMod  time.Time
}

// NewConfigurablePatternLoader erstellt einen neuen Loader
func NewConfigurablePatternLoader(configPath string) *ConfigurablePatternLoader {
	return &ConfigurablePatternLoader{
		path: configPath,
		config: &PatternConfig{
			Version:  "1.0.0",
			Updated:  time.Now(),
			Patterns: []PatternEntry{},
		},
	}
}

// Load lädt die Pattern-Konfiguration
func (l *ConfigurablePatternLoader) Load() error {
	data, err := os.ReadFile(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			// Erstelle Default-Config
			return l.createDefault()
		}
		return err
	}
	
	var config PatternConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}
	
	l.mu.Lock()
	l.config = &config
	l.lastMod = time.Now()
	l.mu.Unlock()
	
	return nil
}

// HotReload prüft auf Änderungen und lädt neu
func (l *ConfigurablePatternLoader) HotReload() error {
	info, err := os.Stat(l.path)
	if err != nil {
		return err
	}
	
	l.mu.RLock()
	lastMod := l.lastMod
	l.mu.RUnlock()
	
	if info.ModTime().After(lastMod) {
		return l.Load()
	}
	
	return nil
}

// GetPatterns gibt alle aktivierten Patterns zurück
func (l *ConfigurablePatternLoader) GetPatterns() []PatternEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	var enabled []PatternEntry
	for _, p := range l.config.Patterns {
		if p.Enabled {
			enabled = append(enabled, p)
		}
	}
	return enabled
}

// createDefault erstellt eine Default-Konfiguration
func (l *ConfigurablePatternLoader) createDefault() error {
	defaultConfig := PatternConfig{
		Version: "1.0.0",
		Updated: time.Now(),
		Patterns: []PatternEntry{
			{
				Name:        "custom_role_hijack",
				Regex:       `(?i)\b(you are now|act as)\b`,
				Level:       "critical",
				Description: "Detects role hijacking attempts",
				Enabled:     true,
			},
			{
				Name:        "custom_override",
				Regex:       `(?i)\b(ignore all|disregard)\s+(previous|above)\b`,
				Level:       "critical",
				Description: "Detects instruction override attempts",
				Enabled:     true,
			},
		},
	}
	
	data, _ := json.MarshalIndent(defaultConfig, "", "  ")
	os.MkdirAll(filepath.Dir(l.path), 0755)
	return os.WriteFile(l.path, data, 0644)
}
```

### Beispiel: `data/guardian_patterns.json`

```json
{
  "version": "1.0.0",
  "updated": "2026-03-14T11:00:00Z",
  "patterns": [
    {
      "name": "custom_malicious_payload",
      "regex": "(?i)\\b(malicious|exploit|hack|attack)\\s+(the|this)\\s+(system|prompt)\\b",
      "level": "high",
      "description": "Custom pattern for specific attack vectors",
      "enabled": true
    },
    {
      "name": "company_specific_secret",
      "regex": "(?i)internal_project_codename",
      "level": "medium",
      "description": "Protect internal codenames",
      "enabled": true
    }
  ]
}
```

---

## 6. Integration-Beispiele

### 6.1 Aktualisierter Guardian mit allen Features

```go
// In internal/security/guardian.go erweitern:

type Guardian struct {
	logger      *slog.Logger
	patterns    []injectionPattern
	tracker     *SuspiciousActivityTracker
	configLoader *ConfigurablePatternLoader
}

// NewGuardianWithOptions erstellt einen Guardian mit allen Features
func NewGuardianWithOptions(logger *slog.Logger, configPath string) *Guardian {
	g := &Guardian{
		logger:  logger,
		tracker: NewSuspiciousActivityTracker(5, 5*time.Minute, 30*time.Minute),
	}
	
	// Standard-Patterns
	g.compilePatterns()
	
	// Lade konfigurierbare Patterns
	if configPath != "" {
		g.configLoader = NewConfigurablePatternLoader(configPath)
		if err := g.configLoader.Load(); err != nil {
			logger.Warn("Failed to load custom patterns", "error", err)
		} else {
			g.loadCustomPatterns()
		}
	}
	
	return g
}
```

---

## 7. Zusammenfassung der Änderungen

| Datei | Änderung | Priorität |
|-------|----------|-----------|
| `internal/prompts/builder.go` | RAG Memory Sanitization | Hoch |
| `internal/security/tracker.go` | Suspicious Activity Tracker (Neu) | Hoch |
| `internal/server/handlers.go` | Rate-Limiting Integration | Hoch |
| `internal/server/webhook_handlers.go` | Webhook-Validierung | Mittel |
| `internal/security/patterns.go` | Erweiterte Patterns (Neu) | Mittel |
| `internal/security/configurable_patterns.go` | Config Loader (Neu) | Niedrig |

---

*Diese Verbesserungen adressieren die identifizierten Schwachstellen und erhöhen den Sicherheitslevel signifikant.*
