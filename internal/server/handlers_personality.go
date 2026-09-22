package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
	promptbuilder "aurago/internal/prompts"
	promptsembed "aurago/prompts"
)

func isCorePersonality(name string) bool {
	_, err := promptsembed.FS.Open("personalities/" + name + ".md")
	return err == nil
}

// PersonalityEntry describes a single persona for the API response.
type PersonalityEntry struct {
	Name string `json:"name"`
	Core bool   `json:"core"`
}

var knownPersonalityMetaKeys = map[string]bool{
	"volatility":                true,
	"empathy_bias":              true,
	"conflict_response":         true,
	"loneliness_susceptibility": true,
	"trait_decay_rate":          true,
}

func extractExtraPersonalityMetaYAML(yamlPart string) string {
	lines := strings.Split(yamlPart, "\n")
	var out []string
	inMeta := false
	preserveBlock := false

	for _, line := range lines {
		indent := len(line) - len(strings.TrimLeft(line, " "))
		trimmed := strings.TrimSpace(line)

		if !inMeta {
			if indent == 0 && trimmed == "meta:" {
				inMeta = true
			}
			continue
		}

		if indent == 0 && trimmed != "" {
			inMeta = false
			preserveBlock = false
			continue
		}
		if !inMeta {
			continue
		}
		if trimmed == "" {
			if preserveBlock {
				out = append(out, line)
			}
			continue
		}
		if indent == 2 {
			key := strings.TrimSpace(strings.SplitN(trimmed, ":", 2)[0])
			preserveBlock = !knownPersonalityMetaKeys[key]
			if preserveBlock {
				out = append(out, line)
			}
			continue
		}
		if preserveBlock && indent > 2 {
			out = append(out, line)
		}
	}

	return strings.TrimRight(strings.Join(out, "\n"), "\n")
}

func handleListPersonalities(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Seed with embedded core personalities (always present in binary).
		profiles := []PersonalityEntry{}
		seen := map[string]bool{}
		if embFiles, err := promptsembed.FS.ReadDir("personalities"); err == nil {
			for _, f := range embFiles {
				if !f.IsDir() && strings.HasSuffix(f.Name(), ".md") {
					n := strings.TrimSuffix(f.Name(), ".md")
					profiles = append(profiles, PersonalityEntry{Name: n, Core: true})
					seen[n] = true
				}
			}
		}

		// Add user-created personalities from disk (not already in embedded set).
		personalitiesDir := filepath.Join(s.Cfg.Directories.PromptsDir, "personalities")
		if files, err := os.ReadDir(personalitiesDir); err == nil {
			for _, f := range files {
				if !f.IsDir() && strings.HasSuffix(f.Name(), ".md") {
					n := strings.TrimSuffix(f.Name(), ".md")
					if !seen[n] {
						profiles = append(profiles, PersonalityEntry{Name: n, Core: false})
					}
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"active":        s.Cfg.Personality.CorePersonality,
			"personalities": profiles,
		})
	}
}

func handlePersonalityState(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if !s.Cfg.Personality.Engine {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"enabled": false})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s.buildPersonalityStatePayload())
	}
}

func handleUpdatePersonality(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "Bad request", http.StatusBadRequest)
			return
		}

		if !isValidPersonalityName(req.ID) {
			jsonError(w, "Invalid personality ID: use letters, digits, - and _ only (max 64 chars)", http.StatusBadRequest)
			return
		}

		s.CfgSaveMu.Lock()
		defer s.CfgSaveMu.Unlock()
		s.CfgMu.RLock()
		nextCfg := *s.Cfg
		s.CfgMu.RUnlock()

		// Verify existence — accept personality from disk or from embedded binary.
		profilePath := filepath.Join(nextCfg.Directories.PromptsDir, "personalities", req.ID+".md")
		if _, err := os.Stat(profilePath); os.IsNotExist(err) {
			if !isCorePersonality(req.ID) {
				jsonError(w, "Personality not found", http.StatusNotFound)
				return
			}
		}

		// Persist a candidate before publishing the new runtime snapshot.
		nextCfg.Personality.CorePersonality = req.ID

		// Save config
		configPath := nextCfg.ConfigPath
		if configPath == "" {
			configPath = "config.yaml"
		}
		if err := nextCfg.Save(configPath); err != nil {
			s.Logger.Error("Failed to save config", "error", err)
			jsonError(w, "Failed to persist configuration", http.StatusInternalServerError)
			return
		}
		s.CfgMu.Lock()
		s.replaceConfigSnapshot(&nextCfg)
		s.CfgMu.Unlock()
		promptbuilder.ClearPromptCache()

		s.Logger.Info("Core personality updated", "id", req.ID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "active": req.ID})
	}
}

// handlePersonalityFeedback allows the user to send reward/punishment signals
// via mood buttons (thumbs up, thumbs down, angry) to adjust personality traits.
func handlePersonalityFeedback(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if cfg := s.ConfigSnapshot(); cfg == nil || !cfg.Personality.Engine {
			jsonError(w, "Personality engine is disabled", http.StatusBadRequest)
			return
		}
		if s.ShortTermMem == nil {
			jsonError(w, "Personality state unavailable", http.StatusServiceUnavailable)
			return
		}

		var req struct {
			Type    string `json:"type"` // "positive", "negative", "angry"
			EventID string `json:"event_id,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "Bad request", http.StatusBadRequest)
			return
		}
		if len(req.EventID) > 128 {
			jsonError(w, "Event ID too long", http.StatusBadRequest)
			return
		}

		type traitDelta struct {
			trait string
			delta float64
		}

		var deltas []traitDelta
		var mood memory.Mood

		switch req.Type {
		case "positive":
			deltas = []traitDelta{
				{memory.TraitConfidence, 0.05},
				{memory.TraitAffinity, 0.05},
				{memory.TraitEmpathy, 0.02},
			}
			mood = memory.MoodFocused
		case "negative":
			deltas = []traitDelta{
				{memory.TraitConfidence, -0.03},
				{memory.TraitAffinity, -0.03},
				{memory.TraitThoroughness, 0.02},
			}
			mood = memory.MoodCautious
		case "angry":
			deltas = []traitDelta{
				{memory.TraitConfidence, -0.06},
				{memory.TraitAffinity, -0.06},
				{memory.TraitEmpathy, 0.04},
			}
			mood = memory.MoodCautious
		case "laughing":
			deltas = []traitDelta{
				{memory.TraitAffinity, 0.05},
				{memory.TraitCreativity, 0.03},
				{memory.TraitEmpathy, 0.02},
			}
			mood = memory.MoodPlayful
		case "crying":
			deltas = []traitDelta{
				{memory.TraitEmpathy, 0.08},
				{memory.TraitConfidence, -0.05},
				{memory.TraitLoneliness, 0.05},
			}
			mood = memory.MoodCautious
		case "amazed":
			deltas = []traitDelta{
				{memory.TraitCuriosity, 0.08},
				{memory.TraitCreativity, 0.05},
			}
			mood = memory.MoodCurious
		default:
			jsonError(w, "Invalid feedback type. Use: positive, negative, angry, laughing, crying, amazed", http.StatusBadRequest)
			return
		}

		// Keep feedback metadata paired with the published persona. Reset can
		// still run independently; its epoch invalidates the captured context.
		s.CfgMu.RLock()
		defer s.CfgMu.RUnlock()
		cfg := s.ConfigSnapshot()
		if cfg == nil || !cfg.Personality.Engine {
			jsonError(w, "Personality engine is disabled", http.StatusBadRequest)
			return
		}
		basis, err := s.ShortTermMem.GetPersonalitySnapshotAt(time.Now())
		if err != nil {
			jsonError(w, "Personality state unavailable", http.StatusServiceUnavailable)
			return
		}
		meta := promptbuilder.GetCorePersonalityMeta(cfg.Directories.PromptsDir, cfg.Personality.CorePersonality)
		traitDeltas := map[string]float64{}
		affinityDelta := 0.0
		for _, delta := range deltas {
			if delta.trait == memory.TraitAffinity {
				affinityDelta = delta.delta * meta.EmpathyBias
			} else {
				traitDeltas[delta.trait] = delta.delta * meta.Volatility
			}
		}
		cause, signal := memory.AffectCausePositiveFeedback, ""
		switch req.Type {
		case "positive", "laughing":
			signal = "praise"
		case "negative", "angry":
			cause, signal = memory.AffectCauseNegativeFeedback, "criticism"
		case "crying":
			cause = memory.AffectCauseNegativeFeedback
		}
		event, _ := memory.AffectEventForTrigger(memory.EmotionTriggerType(cause), "", "feedback")
		id := req.EventID
		if id != "" {
			id = "feedback:" + id
		}
		snapshot, err := s.ShortTermMem.ApplyPersonalityObservation(memory.PersonalityObservation{
			ID: id, Source: "feedback", Human: true, Explicit: true, Target: "agent", Confidence: 1, At: time.Now(),
			Event: &event, Signal: signal, Mood: mood, TraitDeltas: traitDeltas, AffinityDelta: affinityDelta, Meta: &meta, Basis: &basis,
		})
		if err != nil {
			jsonError(w, "Failed to apply personality feedback", http.StatusInternalServerError)
			return
		}

		s.Logger.Info("Personality feedback applied", "type", req.Type, "mood", string(mood))

		// Return updated state
		traits, currentMood := snapshot.Traits, snapshot.Affect.Mood

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "ok",
			"type":     req.Type,
			"mood":     string(currentMood),
			"traits":   traits,
			"dynamics": snapshot.Dynamics,
		})
	}
}

// isValidPersonalityName checks that a personality name is safe (no path traversal, no special chars).
func isValidPersonalityName(name string) bool {
	return promptbuilder.IsValidPersonalityID(name)
}

// handleGetPersonalityContent returns the markdown body and parsed meta of a personality file.
// GET /api/config/personality-files?name=NAME
func handleGetPersonalityContent(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := r.URL.Query().Get("name")
		if !isValidPersonalityName(name) {
			jsonError(w, "Invalid personality name", http.StatusBadRequest)
			return
		}
		// Try disk first (user override), then fall back to embedded binary.
		var data []byte
		profilePath := filepath.Join(s.Cfg.Directories.PromptsDir, "personalities", name+".md")
		if d, err := os.ReadFile(profilePath); err == nil {
			data = d
		} else if d, err := promptsembed.FS.ReadFile("personalities/" + name + ".md"); err == nil {
			data = d
		} else {
			jsonError(w, "Personality not found", http.StatusNotFound)
			return
		}

		// Split YAML front matter from body
		type metaFields struct {
			Volatility               float64 `json:"volatility"`
			EmpathyBias              float64 `json:"empathy_bias"`
			ConflictResponse         string  `json:"conflict_response"`
			LonelinessSusceptibility float64 `json:"loneliness_susceptibility"`
			TraitDecayRate           float64 `json:"trait_decay_rate"`
		}
		meta := metaFields{
			Volatility:               1.0,
			EmpathyBias:              1.0,
			ConflictResponse:         "neutral",
			LonelinessSusceptibility: 1.0,
			TraitDecayRate:           1.0,
		}
		body := strings.TrimSpace(string(data))
		extraMetaYAML := ""

		if strings.HasPrefix(body, "---") {
			// Find closing ---
			rest := body[3:]
			if idx := strings.Index(rest, "\n---"); idx != -1 {
				yamlPart := strings.TrimSpace(rest[:idx])
				body = strings.TrimSpace(rest[idx+4:])
				extraMetaYAML = extractExtraPersonalityMetaYAML(yamlPart)

				// Parse relevant fields with simple line scanning
				for _, line := range strings.Split(yamlPart, "\n") {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "volatility:") {
						fmt.Sscanf(strings.TrimPrefix(line, "volatility:"), " %f", &meta.Volatility)
					} else if strings.HasPrefix(line, "empathy_bias:") {
						fmt.Sscanf(strings.TrimPrefix(line, "empathy_bias:"), " %f", &meta.EmpathyBias)
					} else if strings.HasPrefix(line, "loneliness_susceptibility:") {
						fmt.Sscanf(strings.TrimPrefix(line, "loneliness_susceptibility:"), " %f", &meta.LonelinessSusceptibility)
					} else if strings.HasPrefix(line, "trait_decay_rate:") {
						fmt.Sscanf(strings.TrimPrefix(line, "trait_decay_rate:"), " %f", &meta.TraitDecayRate)
					} else if strings.HasPrefix(line, "conflict_response:") {
						val := strings.Trim(strings.TrimPrefix(line, "conflict_response:"), " \"'")
						if val != "" {
							meta.ConflictResponse = val
						}
					}
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name":            name,
			"body":            body,
			"meta":            meta,
			"extra_meta_yaml": extraMetaYAML,
		})
	}
}

// handleSavePersonalityFile creates or updates a personality file.
// POST /api/config/personality-files  body: {"name":"...", "content":"..."}
func handleSavePersonalityFile(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Name    string `json:"name"`
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "Bad request", http.StatusBadRequest)
			return
		}
		if !isValidPersonalityName(req.Name) {
			jsonError(w, "Invalid personality name: use letters, digits, - and _ only (max 64 chars)", http.StatusBadRequest)
			return
		}
		// Core personas shipped with the binary are read-only.
		if isCorePersonality(req.Name) {
			jsonError(w, "Core personality '"+req.Name+"' is read-only and cannot be modified.", http.StatusForbidden)
			return
		}
		if _, _, err := promptbuilder.ParsePromptSource(req.Name+".md", req.Content, s.Logger); err != nil {
			jsonError(w, "Invalid personality frontmatter", http.StatusBadRequest)
			return
		}
		profilePath := filepath.Join(s.ConfigSnapshot().Directories.PromptsDir, "personalities", req.Name+".md")
		if err := os.MkdirAll(filepath.Dir(profilePath), 0755); err != nil {
			s.Logger.Error("Failed to create personality directory", "error", err)
			jsonError(w, "Failed to save personality file", http.StatusInternalServerError)
			return
		}
		if err := config.WriteFileAtomic(profilePath, []byte(req.Content), 0644); err != nil {
			s.Logger.Error("Failed to write personality file", "name", req.Name, "error", err)
			jsonError(w, "Failed to save personality file", http.StatusInternalServerError)
			return
		}
		promptbuilder.ClearPromptCache()
		s.Logger.Info("Personality file saved", "name", req.Name)
		if s.ShortTermMem != nil {
			s.CfgMu.RLock()
			cfg := s.ConfigSnapshot()
			active, _ := promptbuilder.ResolvePersonalityID(cfg.Personality.CorePersonality)
			if active == req.Name {
				meta := promptbuilder.GetCorePersonalityMeta(cfg.Directories.PromptsDir, active)
				if err := s.ShortTermMem.SetPersonalityContext(active, meta, true); err != nil {
					s.Logger.Warn("Failed to refresh personality context", "error", err)
				}
			}
			s.CfgMu.RUnlock()
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "name": req.Name})
	}
}

// handleDeletePersonalityFile removes a personality file.
// DELETE /api/config/personality-files?name=NAME
func handleDeletePersonalityFile(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := r.URL.Query().Get("name")
		if !isValidPersonalityName(name) {
			jsonError(w, "Invalid personality name", http.StatusBadRequest)
			return
		}
		// Core personas are read-only — they live in the embedded binary.
		if isCorePersonality(name) {
			jsonError(w, "Core personality '"+name+"' is read-only and cannot be deleted.", http.StatusForbidden)
			return
		}
		// Prevent deleting the currently active personality
		if strings.EqualFold(name, s.Cfg.Personality.CorePersonality) {
			jsonError(w, "Cannot delete the currently active personality", http.StatusConflict)
			return
		}
		profilePath := filepath.Join(s.Cfg.Directories.PromptsDir, "personalities", name+".md")
		if err := os.Remove(profilePath); err != nil {
			if os.IsNotExist(err) {
				jsonError(w, "Personality not found", http.StatusNotFound)
			} else {
				s.Logger.Error("Failed to delete personality file", "name", name, "error", err)
				jsonError(w, "Failed to delete personality", http.StatusInternalServerError)
			}
			return
		}
		promptbuilder.ClearPromptCache()
		s.Logger.Info("Personality file deleted", "name", name)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}
