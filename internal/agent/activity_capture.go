package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"

	"github.com/sashabaranov/go-openai"
)

const activityDigestPrompt = `You create compact structured activity digests for an AI agent timeline.

Summarize the handled turn into durable recent-memory fields.
Focus on:
- what the user wanted
- what the agent did
- what happened or was achieved
- important facts/decisions from this turn
- pending follow-ups that still matter

Rules:
- Be concise and concrete.
- Prefer durable wording over chat filler.
- Keep each array item short.
- If there are no pending items, return an empty array.
- importance: 1=minor, 2=normal, 3=important, 4=critical
- entities can contain short identifiers like tools, project names, services, hosts, or files if clearly relevant.

Return ONLY valid JSON in this exact shape:
{"intent":"","user_goal":"","actions_taken":[],"outcomes":[],"important_points":[],"pending_items":[],"importance":2,"entities":[]}

User request:
%s

Assistant reply:
%s

Tools used:
%s

Tool summaries:
%s`

func trackActivityTool(toolNames *[]string, toolSummaries *[]string, action, result string) {
	action = strings.TrimSpace(action)
	if action == "" {
		return
	}
	seen := false
	for _, existing := range *toolNames {
		if existing == action {
			seen = true
			break
		}
	}
	if !seen {
		*toolNames = append(*toolNames, action)
	}
	summary := compactActivityToolResult(action, result)
	if summary != "" {
		*toolSummaries = append(*toolSummaries, summary)
	}
}

func compactActivityToolResult(action, result string) string {
	status := "completed"
	lower := strings.ToLower(result)
	switch {
	case strings.Contains(lower, "permission denied"), strings.Contains(lower, "status\":\"error\""), strings.Contains(lower, "[execution error]"), strings.Contains(lower, "tool output: error"):
		status = "error"
	case strings.Contains(lower, "warning"), strings.Contains(lower, "failed"):
		status = "warning"
	}
	clean := strings.TrimSpace(result)
	clean = strings.TrimPrefix(clean, "Tool Output: ")
	clean = strings.TrimPrefix(clean, "[Tool Output]\n")
	clean = strings.Join(strings.Fields(clean), " ")
	if len(clean) > 180 {
		clean = strings.TrimSpace(clean[:179]) + "…"
	}
	if clean == "" {
		return action + ": " + status
	}
	return action + ": " + status + " - " + clean
}

func captureActivityTurn(cfg *config.Config, logger *slog.Logger, shortTermMem *memory.SQLiteMemory, kg *memory.KnowledgeGraph, sessionID, channel, userRequest, assistantReply string, toolNames, toolSummaries []string, isAutonomous, userRelevant bool) {
	if shortTermMem == nil {
		return
	}
	userRequest = strings.TrimSpace(userRequest)
	assistantReply = strings.TrimSpace(assistantReply)
	if userRequest == "" && assistantReply == "" && len(toolNames) == 0 {
		return
	}

	digest := buildActivityDigest(userRequest, assistantReply, toolNames, toolSummaries, shortTermMem)
	source := "runtime_fallback"
	settings := resolveMemoryAnalysisSettings(cfg, shortTermMem)
	if settings.Enabled && settings.RealTime {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if llmDigest, err := buildActivityDigestWithConfiguredClient(ctx, cfg, userRequest, assistantReply, toolNames, toolSummaries); err == nil {
			digest = llmDigest
			source = "runtime_llm"
		} else if logger != nil {
			logger.Debug("[Activity] Falling back to heuristic digest", "error", err)
		}
	} else {
		source = "runtime"
	}
	persistActivityTurn(shortTermMem, kg, sessionID, channel, userRequest, toolNames, isAutonomous, userRelevant, digest, source)
}

func captureActivityTurnWithDigest(shortTermMem *memory.SQLiteMemory, kg *memory.KnowledgeGraph, sessionID, channel, userRequest string, toolNames []string, isAutonomous, userRelevant bool, digest memory.ActivityDigest, source string) {
	persistActivityTurn(shortTermMem, kg, sessionID, channel, userRequest, toolNames, isAutonomous, userRelevant, digest, source)
}

func persistActivityTurn(shortTermMem *memory.SQLiteMemory, kg *memory.KnowledgeGraph, sessionID, channel, userRequest string, toolNames []string, isAutonomous, userRelevant bool, digest memory.ActivityDigest, source string) {
	if shortTermMem == nil {
		return
	}
	today := time.Now().Format("2006-01-02")
	linkedJournalIDs := make([]int64, 0, 6)
	if entries, err := shortTermMem.GetJournalEntries(today, today, nil, 10); err == nil {
		for _, entry := range entries {
			if entry.SessionID == sessionID {
				linkedJournalIDs = append(linkedJournalIDs, entry.ID)
			}
		}
	}
	linkedNoteIDs := make([]int64, 0, 6)
	if notes, err := shortTermMem.GetHighPriorityOpenNotes(5); err == nil {
		for _, note := range notes {
			linkedNoteIDs = append(linkedNoteIDs, note.ID)
		}
	}

	turnID, err := shortTermMem.InsertActivityTurn(memory.ActivityTurn{
		Timestamp:        time.Now().UTC().Format(time.RFC3339),
		Date:             today,
		SessionID:        sessionID,
		Channel:          channel,
		IsAutonomous:     isAutonomous,
		UserRelevant:     userRelevant,
		Status:           "completed",
		Importance:       digest.Importance,
		Intent:           digest.Intent,
		UserRequest:      userRequest,
		UserGoal:         digest.UserGoal,
		ActionsTaken:     digest.ActionsTaken,
		Outcomes:         digest.Outcomes,
		ImportantPoints:  digest.ImportantPoints,
		PendingItems:     digest.PendingItems,
		ToolNames:        toolNames,
		LinkedJournalIDs: linkedJournalIDs,
		LinkedNoteIDs:    linkedNoteIDs,
		LinkedMemoryIDs:  digest.Entities,
		Source:           source,
	})
	if err == nil && kg != nil {
		syncActivityTurnToKnowledgeGraph(kg, turnID, today, sessionID, channel, digest, source)
	}
}

func syncActivityTurnToKnowledgeGraph(kg *memory.KnowledgeGraph, turnID int64, date, sessionID, channel string, digest memory.ActivityDigest, source string) {
	if kg == nil || turnID <= 0 {
		return
	}

	entities := digest.Entities
	if len(entities) < 1 {
		return
	}

	cleanEntities := make([]string, 0, len(entities))
	for _, raw := range entities {
		label := strings.TrimSpace(raw)
		entityID := normalizeActivityEntityID(label)
		if entityID == "" {
			continue
		}
		_ = kg.AddNode(entityID, label, map[string]string{
			"type":       "activity_entity",
			"source":     "activity_turn",
			"session_id": sessionID,
			"last_seen":  date,
		})
		cleanEntities = append(cleanEntities, entityID)
	}

	for i := 0; i < len(cleanEntities); i++ {
		for j := i + 1; j < len(cleanEntities); j++ {
			a, b := cleanEntities[i], cleanEntities[j]
			if a > b {
				a, b = b, a
			}
			_ = kg.IncrementCoOccurrence(a, b, date)
		}
	}
}

func normalizeActivityEntityID(label string) string {
	label = strings.TrimSpace(strings.ToLower(label))
	if label == "" {
		return ""
	}

	var b strings.Builder
	lastUnderscore := false
	for _, r := range label {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastUnderscore = false
		case !lastUnderscore:
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

func buildActivityDigestWithConfiguredClient(ctx context.Context, cfg *config.Config, userRequest, assistantReply string, toolNames, toolSummaries []string) (memory.ActivityDigest, error) {
	settings := resolveMemoryAnalysisSettings(cfg, nil)
	if !settings.Enabled || !settings.RealTime {
		return memory.ActivityDigest{}, fmt.Errorf("memory analysis disabled")
	}
	llmCfg := resolveMemoryAnalysisLLMConfig(cfg)
	if llmCfg.model == "" {
		return memory.ActivityDigest{}, fmt.Errorf("memory analysis model is empty")
	}
	client := llm.NewClientFromProvider(
		llmCfg.providerType,
		llmCfg.baseURL,
		llmCfg.apiKey,
	)
	model := llmCfg.model
	if model == "" {
		return memory.ActivityDigest{}, fmt.Errorf("memory analysis model is empty")
	}
	return buildActivityDigestWithLLM(ctx, client, model, userRequest, assistantReply, toolNames, toolSummaries)
}

func buildActivityDigestWithLLM(ctx context.Context, client llm.ChatClient, model, userRequest, assistantReply string, toolNames, toolSummaries []string) (memory.ActivityDigest, error) {
	if client == nil {
		return memory.ActivityDigest{}, fmt.Errorf("nil activity digest client")
	}
	prompt := fmt.Sprintf(
		activityDigestPrompt,
		truncateActivityDigestInput(userRequest, 1600),
		truncateActivityDigestInput(assistantReply, 1800),
		strings.Join(uniqueActivityStrings(toolNames, 12), ", "),
		strings.Join(uniqueActivityStrings(toolSummaries, 12), "\n"),
	)
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		Temperature: 0.1,
		MaxTokens:   700,
	})
	if err != nil {
		return memory.ActivityDigest{}, fmt.Errorf("activity digest llm call: %w", err)
	}
	if len(resp.Choices) == 0 || strings.TrimSpace(resp.Choices[0].Message.Content) == "" {
		return memory.ActivityDigest{}, fmt.Errorf("empty activity digest response")
	}
	return parseActivityDigestResponse(resp.Choices[0].Message.Content)
}

func parseActivityDigestResponse(raw string) (memory.ActivityDigest, error) {
	raw = trimJSONResponse(raw)
	var digest memory.ActivityDigest
	if err := json.Unmarshal([]byte(raw), &digest); err != nil {
		return memory.ActivityDigest{}, fmt.Errorf("parse activity digest: %w", err)
	}
	return normalizeActivityDigest(digest), nil
}

func normalizeActivityDigest(digest memory.ActivityDigest) memory.ActivityDigest {
	if digest.Importance < 1 || digest.Importance > 4 {
		digest.Importance = 2
	}
	digest.Intent = truncateActivityDigestInput(digest.Intent, 220)
	if strings.TrimSpace(digest.UserGoal) == "" {
		digest.UserGoal = digest.Intent
	}
	digest.UserGoal = truncateActivityDigestInput(digest.UserGoal, 220)
	digest.ActionsTaken = uniqueActivityStrings(digest.ActionsTaken, 6)
	digest.Outcomes = uniqueActivityStrings(digest.Outcomes, 5)
	digest.ImportantPoints = uniqueActivityStrings(digest.ImportantPoints, 5)
	digest.PendingItems = uniqueActivityStrings(digest.PendingItems, 5)
	digest.Entities = uniqueActivityStrings(digest.Entities, 8)
	return digest
}

func buildActivityDigest(userRequest, assistantReply string, toolNames, toolSummaries []string, shortTermMem *memory.SQLiteMemory) memory.ActivityDigest {
	intent := userRequest
	if intent == "" {
		intent = "Autonomous follow-up"
	}
	if len(intent) > 200 {
		intent = strings.TrimSpace(intent[:199]) + "…"
	}
	actions := make([]string, 0, len(toolSummaries)+1)
	if len(toolSummaries) > 0 {
		actions = append(actions, toolSummaries...)
	} else if len(toolNames) > 0 {
		actions = append(actions, strings.Join(toolNames, ", "))
	} else {
		actions = append(actions, "Responded directly in chat")
	}

	outcomes := make([]string, 0, 4)
	if assistantReply != "" {
		reply := strings.Join(strings.Fields(assistantReply), " ")
		if len(reply) > 240 {
			reply = strings.TrimSpace(reply[:239]) + "…"
		}
		outcomes = append(outcomes, reply)
	}
	if len(toolSummaries) > 0 {
		outcomes = append(outcomes, toolSummaries...)
	}

	important := make([]string, 0, 6)
	if userRequest != "" {
		important = append(important, "User request: "+intent)
	}
	if len(toolNames) > 0 {
		important = append(important, "Tools used: "+strings.Join(toolNames, ", "))
	}

	pending := make([]string, 0, 5)
	if shortTermMem != nil {
		if notes, err := shortTermMem.GetHighPriorityOpenNotes(5); err == nil {
			for _, note := range notes {
				pending = append(pending, note.Title)
			}
		}
	}

	importance := 1
	if len(toolNames) >= 3 {
		importance = 3
	} else if len(toolNames) > 0 || len(strings.Fields(assistantReply)) > 40 {
		importance = 2
	}
	replyLower := strings.ToLower(assistantReply)
	if strings.Contains(replyLower, "todo") || strings.Contains(replyLower, "next") || strings.Contains(replyLower, "offen") {
		importance = 3
	}

	return memory.ActivityDigest{
		Intent:          intent,
		UserGoal:        intent,
		ActionsTaken:    uniqueActivityStrings(actions, 6),
		Outcomes:        uniqueActivityStrings(outcomes, 5),
		ImportantPoints: uniqueActivityStrings(important, 5),
		PendingItems:    uniqueActivityStrings(pending, 5),
		Importance:      importance,
		Entities:        uniqueActivityStrings(toolNames, 8),
	}
}

func uniqueActivityStrings(items []string, limit int) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func truncateActivityDigestInput(text string, maxLen int) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if maxLen <= 0 || len(text) <= maxLen {
		return text
	}
	return strings.TrimSpace(text[:maxLen-1]) + "…"
}
