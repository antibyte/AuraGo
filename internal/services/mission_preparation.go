package services

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

// MissionPreparationService analyses missions via LLM and caches structured
// preparation context (tool guides, step plans, pitfalls) for later injection
// into the mission execution prompt.
type MissionPreparationService struct {
	cfg        *config.Config
	cfgMu      *sync.RWMutex
	db         *sql.DB
	missionMgr *tools.MissionManagerV2
	logger     *slog.Logger
	cancel     context.CancelFunc
	promptTpl  string // loaded from prompts/mission_preparation.md

	// availableTools is a list of tool names the agent can use.
	// Set via SetAvailableTools before Start.
	availableTools []string
}

// NewMissionPreparationService creates a new preparation service.
func NewMissionPreparationService(
	cfg *config.Config,
	cfgMu *sync.RWMutex,
	db *sql.DB,
	missionMgr *tools.MissionManagerV2,
	logger *slog.Logger,
) *MissionPreparationService {
	return &MissionPreparationService{
		cfg:        cfg,
		cfgMu:      cfgMu,
		db:         db,
		missionMgr: missionMgr,
		logger:     logger,
	}
}

// SetAvailableTools provides the list of tool names available to the agent.
func (s *MissionPreparationService) SetAvailableTools(names []string) {
	s.availableTools = names
}

// Start loads the prompt template and starts the background loop that
// auto-prepares eligible missions.
func (s *MissionPreparationService) Start(ctx context.Context) {
	ctx, s.cancel = context.WithCancel(ctx)

	// Load prompt template
	s.cfgMu.RLock()
	promptsDir := s.cfg.Directories.PromptsDir
	s.cfgMu.RUnlock()
	if promptsDir == "" {
		promptsDir = "prompts"
	}

	tplData, err := os.ReadFile(promptsDir + "/mission_preparation.md")
	if err != nil {
		s.logger.Warn("[MissionPrep] Could not load prompt template, using built-in", "error", err)
		s.promptTpl = defaultPrepPrompt
	} else {
		s.promptTpl = string(tplData)
	}

	s.logger.Info("[MissionPrep] Mission preparation service started")

	go s.loop(ctx)
}

// Stop cancels the background loop.
func (s *MissionPreparationService) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.logger.Info("[MissionPrep] Mission preparation service stopped")
}

// loop periodically checks for missions that need (re-)preparation.
func (s *MissionPreparationService) loop(ctx context.Context) {
	// Initial sweep after a short delay to let missions load.
	time.Sleep(10 * time.Second)
	s.autoPrepareEligible(ctx)

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.autoPrepareEligible(ctx)
		}
	}
}

// autoPrepareEligible finds scheduled missions with auto_prepare enabled
// that need (re-)preparation and prepares them.
func (s *MissionPreparationService) autoPrepareEligible(ctx context.Context) {
	s.cfgMu.RLock()
	enabled := s.cfg.MissionPreparation.Enabled
	autoPrepScheduled := s.cfg.MissionPreparation.AutoPrepareScheduled
	s.cfgMu.RUnlock()

	if !enabled {
		return
	}

	missions := s.missionMgr.List()
	for _, m := range missions {
		if ctx.Err() != nil {
			return
		}
		if !m.Enabled {
			continue
		}
		// Auto-prepare: scheduled missions with global flag, or any mission with auto_prepare
		shouldPrepare := (m.ExecutionType == "scheduled" && autoPrepScheduled) || m.AutoPrepare
		if !shouldPrepare {
			continue
		}

		// Check current status
		existing, err := tools.GetPreparedMission(s.db, m.ID)
		if err != nil {
			s.logger.Error("[MissionPrep] Failed to check preparation status", "mission", m.ID, "error", err)
			continue
		}

		// Skip if already prepared and not stale
		if existing != nil && existing.Status == tools.PrepStatusPrepared {
			// Check if checksum still matches (mission not changed)
			checksum := s.computeChecksum(m)
			if existing.SourceChecksum == checksum {
				continue
			}
			// Source changed → re-prepare
		}

		if _, err := s.PrepareMission(ctx, m.ID); err != nil {
			s.logger.Error("[MissionPrep] Auto-prepare failed", "mission", m.ID, "error", err)
		}
	}

	// Cleanup expired preparations
	s.cfgMu.RLock()
	expiryHours := s.cfg.MissionPreparation.CacheExpiryHours
	s.cfgMu.RUnlock()
	if expiryHours > 0 {
		if n, err := tools.CleanupExpiredPreparations(s.db, time.Duration(expiryHours)*time.Hour); err != nil {
			s.logger.Error("[MissionPrep] Cleanup failed", "error", err)
		} else if n > 0 {
			s.logger.Info("[MissionPrep] Cleaned up expired preparations", "count", n)
		}
	}
}

// PrepareMission runs the LLM analysis for a single mission and stores the result.
func (s *MissionPreparationService) PrepareMission(ctx context.Context, missionID string) (*tools.PreparedMission, error) {
	mission, ok := s.missionMgr.Get(missionID)
	if !ok {
		return nil, fmt.Errorf("mission not found: %s", missionID)
	}

	s.cfgMu.RLock()
	prepCfg := s.cfg.MissionPreparation
	s.cfgMu.RUnlock()

	if !prepCfg.Enabled {
		return nil, fmt.Errorf("mission preparation is disabled")
	}

	// Compute source checksum
	checksum := s.computeChecksum(mission)

	// Check cache: if already prepared with same checksum → return cached
	existing, err := tools.GetPreparedMission(s.db, missionID)
	if err != nil {
		return nil, fmt.Errorf("failed to check cache: %w", err)
	}
	if existing != nil && existing.Status == tools.PrepStatusPrepared && existing.SourceChecksum == checksum {
		s.logger.Info("[MissionPrep] Cache hit, skipping re-preparation", "mission", missionID)
		return existing, nil
	}

	// Mark as preparing
	s.missionMgr.SetPreparationStatus(missionID, string(tools.PrepStatusPreparing))

	start := time.Now()

	// Build the LLM prompt
	systemPrompt := s.buildSystemPrompt(prepCfg.MaxEssentialTools)
	userPrompt := s.buildUserPrompt(mission)

	// Create LLM client
	apiKey := prepCfg.APIKey
	if apiKey == "" {
		return nil, fmt.Errorf("no API key configured for mission preparation")
	}

	clientCfg := openai.DefaultConfig(apiKey)
	if prepCfg.BaseURL != "" {
		url := strings.TrimRight(prepCfg.BaseURL, "/")
		if !strings.Contains(url, "/v1") {
			url += "/v1"
		}
		clientCfg.BaseURL = url
	}
	client := openai.NewClientWithConfig(clientCfg)

	timeout := time.Duration(prepCfg.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 120 * time.Second
	}
	llmCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req := openai.ChatCompletionRequest{
		Model: prepCfg.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
		Temperature:    0.3,
		ResponseFormat: &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeJSONObject},
	}

	resp, err := client.CreateChatCompletion(llmCtx, req)
	elapsed := time.Since(start)

	if err != nil {
		// Record error
		pm := &tools.PreparedMission{
			ID:                fmt.Sprintf("prep_%s", missionID),
			MissionID:         missionID,
			Version:           1,
			Status:            tools.PrepStatusError,
			PreparedAt:        time.Now(),
			SourceChecksum:    checksum,
			PreparationTimeMS: elapsed.Milliseconds(),
			ErrorMessage:      err.Error(),
		}
		tools.SavePreparedMission(s.db, pm)
		s.missionMgr.SetPreparationStatus(missionID, string(tools.PrepStatusError))
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		s.missionMgr.SetPreparationStatus(missionID, string(tools.PrepStatusError))
		return nil, fmt.Errorf("LLM returned no choices")
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content)

	// Parse JSON response
	var analysis tools.PreparationAnalysis
	var confidence float64

	// Try to parse the response which may have a top-level "confidence" field
	var rawResponse struct {
		tools.PreparationAnalysis
		Confidence float64 `json:"confidence"`
	}
	if err := json.Unmarshal([]byte(content), &rawResponse); err != nil {
		pm := &tools.PreparedMission{
			ID:                fmt.Sprintf("prep_%s", missionID),
			MissionID:         missionID,
			Version:           1,
			Status:            tools.PrepStatusError,
			PreparedAt:        time.Now(),
			SourceChecksum:    checksum,
			PreparationTimeMS: elapsed.Milliseconds(),
			ErrorMessage:      fmt.Sprintf("failed to parse LLM response: %s", err),
		}
		tools.SavePreparedMission(s.db, pm)
		s.missionMgr.SetPreparationStatus(missionID, string(tools.PrepStatusError))
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	analysis = rawResponse.PreparationAnalysis
	confidence = rawResponse.Confidence

	// Determine status based on confidence
	status := tools.PrepStatusPrepared
	if confidence < prepCfg.MinConfidence {
		status = tools.PrepStatusLowConfidence
	}

	// Count tokens
	tokenCost := 0
	if resp.Usage.TotalTokens > 0 {
		tokenCost = resp.Usage.TotalTokens
	}

	pm := &tools.PreparedMission{
		ID:                fmt.Sprintf("prep_%s", missionID),
		MissionID:         missionID,
		Version:           1,
		Status:            status,
		PreparedAt:        time.Now(),
		SourceChecksum:    checksum,
		Confidence:        confidence,
		TokenCost:         tokenCost,
		PreparationTimeMS: elapsed.Milliseconds(),
		Analysis:          &analysis,
	}

	if err := tools.SavePreparedMission(s.db, pm); err != nil {
		s.missionMgr.SetPreparationStatus(missionID, string(tools.PrepStatusError))
		return nil, fmt.Errorf("failed to save preparation: %w", err)
	}

	s.missionMgr.SetPreparationStatus(missionID, string(status))

	s.logger.Info("[MissionPrep] Mission prepared",
		"mission", missionID,
		"confidence", confidence,
		"tokens", tokenCost,
		"elapsed_ms", elapsed.Milliseconds())

	return pm, nil
}

// InvalidateMission marks a mission's preparation as stale.
func (s *MissionPreparationService) InvalidateMission(missionID string) {
	if err := tools.InvalidatePreparedMission(s.db, missionID); err != nil {
		s.logger.Error("[MissionPrep] Failed to invalidate", "mission", missionID, "error", err)
	}
	s.missionMgr.SetPreparationStatus(missionID, string(tools.PrepStatusStale))
}

// InvalidateByCheatsheet invalidates all preparations that reference a cheatsheet.
func (s *MissionPreparationService) InvalidateByCheatsheet(cheatsheetID string) {
	if err := tools.InvalidatePreparedMissionsByCheatsheet(s.db, s.missionMgr, cheatsheetID); err != nil {
		s.logger.Error("[MissionPrep] Failed to invalidate by cheatsheet", "cheatsheet", cheatsheetID, "error", err)
	}
}

// GetPreparedContext returns the prepared mission for injection into processNext.
func (s *MissionPreparationService) GetPreparedContext(missionID string) *tools.PreparedMission {
	pm, err := tools.GetPreparedMission(s.db, missionID)
	if err != nil {
		s.logger.Error("[MissionPrep] Failed to get prepared context", "mission", missionID, "error", err)
		return nil
	}
	return pm
}

// computeChecksum returns a SHA256 hex digest of the mission prompt + cheatsheet IDs.
func (s *MissionPreparationService) computeChecksum(mission *tools.MissionV2) string {
	h := sha256.New()
	h.Write([]byte(mission.Prompt))
	for _, id := range mission.CheatsheetIDs {
		h.Write([]byte(id))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// buildSystemPrompt returns the system prompt with template values filled in.
func (s *MissionPreparationService) buildSystemPrompt(maxTools int) string {
	prompt := s.promptTpl
	prompt = strings.ReplaceAll(prompt, "{{.MaxEssentialTools}}", fmt.Sprintf("%d", maxTools))
	return prompt
}

// buildUserPrompt constructs the user message with mission details and available tools.
func (s *MissionPreparationService) buildUserPrompt(mission *tools.MissionV2) string {
	var sb strings.Builder

	sb.WriteString("## Mission Prompt\n")
	sb.WriteString(mission.Prompt)
	sb.WriteString("\n\n")

	// Attach cheatsheet content if available
	if len(mission.CheatsheetIDs) > 0 {
		cheatsheetDB := s.missionMgr.GetCheatsheetDB()
		if cheatsheetDB != nil {
			if extra := tools.CheatsheetGetMultiple(cheatsheetDB, mission.CheatsheetIDs); extra != "" {
				sb.WriteString("## Reference Material (Cheatsheets)\n")
				sb.WriteString(extra)
				sb.WriteString("\n\n")
			}
		}
	}

	// Available tools
	if len(s.availableTools) > 0 {
		sb.WriteString("## Available Tools\n")
		for _, t := range s.availableTools {
			sb.WriteString("- ")
			sb.WriteString(t)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// defaultPrepPrompt is used when the template file cannot be loaded.
const defaultPrepPrompt = `You are a mission preparation analyst. Analyze the mission prompt and produce a JSON preparation guide with: summary, essential_tools, step_plan, decision_points, pitfalls, preloads, estimated_steps, confidence. Respond with valid JSON only.`
