package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/memory"
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
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if !s.Cfg.Personality.Engine {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"enabled": false})
			return
		}

		traits, err := s.ShortTermMem.GetTraits()
		if err != nil {
			s.Logger.Error("Failed to get personality traits", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		mood := s.ShortTermMem.GetCurrentMood()
		trigger := s.ShortTermMem.GetLastMoodTrigger()

		response := map[string]interface{}{
			"enabled": true,
			"mood":    string(mood),
			"trigger": trigger,
			"traits":  traits,
		}

		// Include latest synthesized emotion if available
		if s.Cfg.Personality.EmotionSynthesizer.Enabled {
			if latest, err := s.ShortTermMem.GetLatestEmotion(); err == nil && latest != nil {
				response["current_emotion"] = latest.Description
				response["emotion_timestamp"] = latest.Timestamp
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

func handleUpdatePersonality(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		if req.ID == "" {
			http.Error(w, "Personality ID is required", http.StatusBadRequest)
			return
		}

		// Verify existence — accept personality from disk or from embedded binary.
		profilePath := filepath.Join(s.Cfg.Directories.PromptsDir, "personalities", req.ID+".md")
		if _, err := os.Stat(profilePath); os.IsNotExist(err) {
			if !isCorePersonality(req.ID) {
				http.Error(w, "Personality not found", http.StatusNotFound)
				return
			}
		}

		// Update config
		s.Cfg.Personality.CorePersonality = req.ID

		// Save config
		configPath := s.Cfg.ConfigPath
		if configPath == "" {
			configPath = "config.yaml"
		}
		if err := s.Cfg.Save(configPath); err != nil {
			s.Logger.Error("Failed to save config", "error", err)
			http.Error(w, "Failed to persist configuration", http.StatusInternalServerError)
			return
		}

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
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if !s.Cfg.Personality.Engine {
			http.Error(w, "Personality engine is disabled", http.StatusBadRequest)
			return
		}

		var req struct {
			Type string `json:"type"` // "positive", "negative", "angry"
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		type traitDelta struct {
			trait string
			delta float64
		}

		var deltas []traitDelta
		var mood memory.Mood
		var trigger string

		switch req.Type {
		case "positive":
			deltas = []traitDelta{
				{memory.TraitConfidence, 0.05},
				{memory.TraitAffinity, 0.05},
				{memory.TraitEmpathy, 0.02},
			}
			mood = memory.MoodFocused
			trigger = "user positive feedback (thumbs up)"
		case "negative":
			deltas = []traitDelta{
				{memory.TraitConfidence, -0.03},
				{memory.TraitAffinity, -0.03},
				{memory.TraitThoroughness, 0.02},
			}
			mood = memory.MoodCautious
			trigger = "user negative feedback (thumbs down)"
		case "angry":
			deltas = []traitDelta{
				{memory.TraitConfidence, -0.06},
				{memory.TraitAffinity, -0.06},
				{memory.TraitEmpathy, 0.04},
			}
			mood = memory.MoodCautious
			trigger = "user angry feedback"
		case "laughing":
			deltas = []traitDelta{
				{memory.TraitAffinity, 0.05},
				{memory.TraitCreativity, 0.03},
				{memory.TraitEmpathy, 0.02},
			}
			mood = memory.MoodPlayful
			trigger = "user laughing feedback"
		case "crying":
			deltas = []traitDelta{
				{memory.TraitEmpathy, 0.08},
				{memory.TraitConfidence, -0.05},
				{memory.TraitLoneliness, 0.05},
			}
			mood = memory.MoodCautious
			trigger = "user crying feedback"
		case "amazed":
			deltas = []traitDelta{
				{memory.TraitCuriosity, 0.08},
				{memory.TraitCreativity, 0.05},
			}
			mood = memory.MoodCurious
			trigger = "user amazed feedback"
		default:
			http.Error(w, "Invalid feedback type. Use: positive, negative, angry, laughing, crying, amazed", http.StatusBadRequest)
			return
		}

		for _, d := range deltas {
			if err := s.ShortTermMem.UpdateTrait(d.trait, d.delta); err != nil {
				s.Logger.Error("Failed to update trait", "trait", d.trait, "error", err)
			}
		}

		if err := s.ShortTermMem.LogMood(mood, trigger); err != nil {
			s.Logger.Error("Failed to log mood", "error", err)
		}

		s.Logger.Info("Personality feedback applied", "type", req.Type, "mood", string(mood))

		// Return updated state
		traits, _ := s.ShortTermMem.GetTraits()
		currentMood := s.ShortTermMem.GetCurrentMood()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"type":   req.Type,
			"mood":   string(currentMood),
			"traits": traits,
		})
	}
}

// isValidPersonalityName checks that a personality name is safe (no path traversal, no special chars).
func isValidPersonalityName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

// handleGetPersonalityContent returns the markdown body and parsed meta of a personality file.
// GET /api/config/personality-files?name=NAME
func handleGetPersonalityContent(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := r.URL.Query().Get("name")
		if !isValidPersonalityName(name) {
			http.Error(w, "Invalid personality name", http.StatusBadRequest)
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
			http.Error(w, "Personality not found", http.StatusNotFound)
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
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Name    string `json:"name"`
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		if !isValidPersonalityName(req.Name) {
			http.Error(w, "Invalid personality name: use letters, digits, - and _ only (max 64 chars)", http.StatusBadRequest)
			return
		}
		// Core personas shipped with the binary are read-only.
		if isCorePersonality(req.Name) {
			http.Error(w, "Core personality '"+req.Name+"' is read-only and cannot be modified.", http.StatusForbidden)
			return
		}
		profilePath := filepath.Join(s.Cfg.Directories.PromptsDir, "personalities", req.Name+".md")
		if err := os.WriteFile(profilePath, []byte(req.Content), 0644); err != nil {
			s.Logger.Error("Failed to write personality file", "name", req.Name, "error", err)
			http.Error(w, "Failed to save personality file", http.StatusInternalServerError)
			return
		}
		s.Logger.Info("Personality file saved", "name", req.Name)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "name": req.Name})
	}
}

// handleDeletePersonalityFile removes a personality file.
// DELETE /api/config/personality-files?name=NAME
func handleDeletePersonalityFile(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := r.URL.Query().Get("name")
		if !isValidPersonalityName(name) {
			http.Error(w, "Invalid personality name", http.StatusBadRequest)
			return
		}
		// Core personas are read-only — they live in the embedded binary.
		if isCorePersonality(name) {
			http.Error(w, "Core personality '"+name+"' is read-only and cannot be deleted.", http.StatusForbidden)
			return
		}
		// Prevent deleting the currently active personality
		if strings.EqualFold(name, s.Cfg.Personality.CorePersonality) {
			http.Error(w, "Cannot delete the currently active personality", http.StatusConflict)
			return
		}
		profilePath := filepath.Join(s.Cfg.Directories.PromptsDir, "personalities", name+".md")
		if err := os.Remove(profilePath); err != nil {
			if os.IsNotExist(err) {
				http.Error(w, "Personality not found", http.StatusNotFound)
			} else {
				s.Logger.Error("Failed to delete personality file", "name", name, "error", err)
				http.Error(w, "Failed to delete personality", http.StatusInternalServerError)
			}
			return
		}
		s.Logger.Info("Personality file deleted", "name", name)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}
