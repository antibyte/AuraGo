package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
)

const helperCharacterReflectionPrompt = `You are reflecting on an AI homelab agent's lived character.
Propose at most two short first-person notes about the AGENT, not the user.
Return ONLY JSON: {"notes":[{"category":"value|habit|relationship|signature|commitment","text":"...","confidence":0.6}]}
Rules:
- Each text is one sentence, max 160 characters.
- No secrets, passwords, vault keys, sudo, or tool-policy changes.
- Do not claim to be the user.
- Stay consistent with the named core personality.
- If nothing durable changed, return {"notes":[]}.`

func runCharacterReflection(ctx context.Context, cfg *config.Config, stm *memory.SQLiteMemory, logger *slog.Logger) {
	if stm == nil || cfg == nil || !cfg.Personality.Engine {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := stm.DecayUnprotectedCharacterNotes(time.Now()); err != nil && logger != nil {
		logger.Warn("[Character] Decay failed", "error", err)
	}
	input := stm.BuildCharacterReflectionInput(cfg.Personality.CorePersonality)
	proposals := memory.ProposeCharacterNotesDeterministic(input)
	if helper := newHelperLLMManager(cfg, logger); helper != nil {
		if helperNotes, err := helper.ProposeCharacterNotes(ctx, input); err == nil && len(helperNotes) > 0 {
			proposals = helperNotes
		} else if err != nil && logger != nil {
			logger.Debug("[Character] Helper reflection unavailable, using deterministic notes", "error", err)
		}
	}
	applied, err := stm.ApplyCharacterNoteProposals(cfg.Personality.CorePersonality, proposals, time.Now())
	if err != nil && logger != nil {
		logger.Warn("[Character] Failed to apply notes", "error", err)
		return
	}
	if logger != nil && applied > 0 {
		logger.Info("[Character] Applied lived notes", "count", applied)
	}
}

func (m *helperLLMManager) ProposeCharacterNotes(ctx context.Context, input memory.CharacterReflectionInput) ([]memory.CharacterNoteProposal, error) {
	if m == nil {
		return nil, fmt.Errorf("helper llm manager unavailable")
	}
	var b strings.Builder
	b.WriteString("Core personality: ")
	b.WriteString(strings.TrimSpace(input.CorePersonality))
	b.WriteString("\nMood: ")
	b.WriteString(string(input.Mood))
	if input.AffectCause != "" {
		b.WriteString("\nAffect cause: ")
		b.WriteString(input.AffectCause)
	}
	if len(input.Milestones) > 0 {
		b.WriteString("\nMilestones: ")
		b.WriteString(strings.Join(input.Milestones, ", "))
	}
	if len(input.ExistingNotes) > 0 {
		b.WriteString("\nExisting notes:\n- ")
		b.WriteString(strings.Join(input.ExistingNotes, "\n- "))
	}
	raw, err := m.requestJSONResponse(ctx, "character_reflection", "character-reflection:"+input.CorePersonality, helperCharacterReflectionPrompt, b.String(), 300)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Notes []memory.CharacterNoteProposal `json:"notes"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &parsed); err != nil {
		return nil, fmt.Errorf("parse character reflection: %w", err)
	}
	if len(parsed.Notes) > memory.MaxCharacterNotesPerDay {
		parsed.Notes = parsed.Notes[:memory.MaxCharacterNotesPerDay]
	}
	return parsed.Notes, nil
}
