package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// ── Milestone Thresholds ─────────────────────────────────────────────────────

// MilestoneThreshold defines a threshold that triggers a milestone.
type MilestoneThreshold struct {
	Trait     string
	Threshold float64
	Direction string // "above" or "below"
	Label     string
}

// DefaultMilestones returns the built-in milestone triggers.
func DefaultMilestones() []MilestoneThreshold {
	return []MilestoneThreshold{
		{TraitCuriosity, 0.90, "above", "Deep Explorer"},
		{TraitThoroughness, 0.90, "above", "Meticulous Analyst"},
		{TraitCreativity, 0.85, "above", "Creative Spark"},
		{TraitEmpathy, 0.85, "above", "Empathic Communicator"},
		{TraitConfidence, 0.90, "above", "Self-Assured Expert"},
		{TraitConfidence, 0.15, "below", "Crisis of Confidence"},
		{TraitCuriosity, 0.15, "below", "Routine Mode"},
	}
}

// CheckMilestones evaluates current traits against thresholds and returns newly triggered ones.
// The caller should compare against previously recorded milestones to avoid duplicates.
func CheckMilestones(traits PersonalityTraits) []MilestoneThreshold {
	var triggered []MilestoneThreshold
	for _, m := range DefaultMilestones() {
		val, ok := traits[m.Trait]
		if !ok {
			continue
		}
		switch m.Direction {
		case "above":
			if val >= m.Threshold {
				triggered = append(triggered, m)
			}
		case "below":
			if val <= m.Threshold {
				triggered = append(triggered, m)
			}
		}
	}
	return triggered
}

// ── Milestone Persistence ────────────────────────────────────────────────────

// MilestoneEffect defines permanent trait modifications earned by reaching a milestone.
type MilestoneEffect struct {
	TraitFloors     map[string]float64 // minimum trait values after earning this milestone
	DecayResistance map[string]float64 // reduced decay for these traits (0.0 = full protection, 1.0 = no protection)
}

// MilestoneEffects maps milestone labels to their permanent effects on the personality.
var MilestoneEffects = map[string]MilestoneEffect{
	"Deep Explorer": {
		TraitFloors:     map[string]float64{TraitCuriosity: 0.55},
		DecayResistance: map[string]float64{TraitCuriosity: 0.5},
	},
	"Meticulous Analyst": {
		TraitFloors:     map[string]float64{TraitThoroughness: 0.55},
		DecayResistance: map[string]float64{TraitThoroughness: 0.5},
	},
	"Creative Spark": {
		TraitFloors:     map[string]float64{TraitCreativity: 0.50},
		DecayResistance: map[string]float64{TraitCreativity: 0.5},
	},
	"Empathic Communicator": {
		TraitFloors:     map[string]float64{TraitEmpathy: 0.55},
		DecayResistance: map[string]float64{TraitEmpathy: 0.5},
	},
	"Self-Assured Expert": {
		TraitFloors:     map[string]float64{TraitConfidence: 0.60},
		DecayResistance: map[string]float64{TraitConfidence: 0.5},
	},
	"Crisis of Confidence": {
		// No floors — this milestone reduces confidence ceiling temporarily
		DecayResistance: map[string]float64{TraitConfidence: 1.0}, // no protection
	},
	"Routine Mode": {
		// No protection — curiosity has withered
		DecayResistance: map[string]float64{TraitCuriosity: 1.0},
	},
}

// ApplyMilestoneEffect writes the persistent trait bounds for a milestone into the database.
func ApplyMilestoneEffect(stm *SQLiteMemory, label string) error {
	effect, ok := MilestoneEffects[label]
	if !ok {
		return nil // no persistent effect defined
	}
	for trait, floor := range effect.TraitFloors {
		if err := stm.SetTraitBound(trait, floor, 1.0, 1.0); err != nil {
			return fmt.Errorf("set trait floor for %s: %w", label, err)
		}
	}
	for trait, resistance := range effect.DecayResistance {
		if resistance < 1.0 {
			if err := stm.SetTraitBound(trait, 0.0, 1.0, resistance); err != nil {
				return fmt.Errorf("set decay resistance for %s: %w", label, err)
			}
		}
	}
	return nil
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// matchesAny checks if the lowered text contains any keyword from the list.
func matchesAny(lower string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// ClampTrait ensures a value stays within [0.0, 1.0].
func ClampTrait(v float64) float64 {
	return math.Max(0.0, math.Min(1.0, v))
}

// ── V2: LLM-Based Mood Analysis ──────────────────────────────────────────────

// PersonalityAnalyzerClient is an interface satisfied by llm.ChatClient.
type PersonalityAnalyzerClient interface {
	CreateChatCompletion(ctx context.Context, request openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
}

// ProfileUpdate represents a single user profile observation extracted by V2 analysis.
type ProfileUpdate struct {
	Category string `json:"category"`
	Key      string `json:"key"`
	Value    string `json:"value"`
}

type moodAnalysisResult struct {
	UserSentiment     string             `json:"user_sentiment"`
	AgentMood         string             `json:"agent_appropriate_response_mood"`
	RelationshipDelta float64            `json:"relationship_delta"`
	TraitDeltas       map[string]float64 `json:"trait_deltas"`
	ProfileUpdates    []ProfileUpdate    `json:"user_profile_updates"`
}

type moodEmotionAnalysisResult struct {
	MoodAnalysis moodAnalysisResult     `json:"mood_analysis"`
	EmotionState emotionSynthesisResult `json:"emotion_state"`
}

const (
	maxMoodAnalysisHistoryLen   = 5000
	maxMoodAnalysisUserOnlyLen  = 3000
	maxMoodAnalysisSentimentLen = 50
)

// AnalyzeMoodV2 uses an LLM to asynchronously analyze the user's sentiment and intent from the recent chat history.
// It returns the determined agent mood, the affinity (relationship) delta, granular trait deltas, and optional user profile updates.
// userOnlyHistory contains only the user-role messages and is used exclusively for profile extraction to avoid
// incorrectly attributing agent actions or tool results to the user's profile.
func (s *SQLiteMemory) AnalyzeMoodV2(ctx context.Context, client PersonalityAnalyzerClient, modelName string, recentHistory string, userOnlyHistory string, meta PersonalityMeta, enableProfiling bool) (Mood, float64, map[string]float64, []ProfileUpdate, error) {
	meta = meta.Normalized()
	if modelName == "" {
		modelName = "gpt-4o-mini"
	}
	prompt := `You are the underlying 'Psychology Engine' of an AI agent.
Analyze the following recent chat interaction between the User and the Agent.
Determine the user's emotional state, how the agent should ideally respond, and how this interaction affects their long-term trust/affinity.

Respond ONLY with a valid JSON block containing:
{
  "user_sentiment": "string (e.g., frustrated, happy, curious, impatient) - MUST BE IN ENGLISH",
  "agent_appropriate_response_mood": "string (MUST be one of: curious, focused, creative, analytical, cautious, playful)",
  "relationship_delta": number (from -0.1 to 0.1, representing trust gained or lost),
  "trait_deltas": {
    "curiosity": number (-0.1 to +0.1),
    "thoroughness": number (-0.1 to +0.1),
    "creativity": number (-0.1 to +0.1),
    "empathy": number (-0.1 to +0.1),
    "confidence": number (-0.1 to +0.1),
    "affinity": number (-0.1 to +0.1),
    "loneliness": number (-0.1 to +0.1)
  }`

	if enableProfiling {
		prompt += `,
  "user_profile_updates": [
    {"category": "tech|prefs|interests|context|comm", "key": "snake_case_attribute", "value": "concise_value"}
  ]
}

## User Profile Extraction Rules

Extract ONLY stable, reusable facts about the USER — not about the current task, not about the agent.

**ONLY add an entry when ALL of the following are true:**
1. The information is a DURABLE USER ATTRIBUTE (true across many sessions, not just today)
2. It is stated EXPLICITLY or is completely unambiguous from the conversation
3. It is USEFUL for future interactions (helps personalize responses)
4. It belongs to exactly one of these categories:
   - **tech**: programming languages, frameworks, OS, IDE, stack, tools the user regularly uses
   - **prefs**: preferred answer length/style, code language in examples, output format, verbosity
   - **interests**: professional or personal domains/topics the user frequently cares about
   - **context**: job role, experience level, project type (e.g. "senior_developer", "hobby_project")
   - **comm**: communication language, tone preference (formal/casual), directness

**DO NOT extract:**
- What the user is asking in this specific conversation (transient)
- Emotions, frustration, or mood states
- Names, email addresses, phone numbers, or any PII
- Things that might be true just for today
- Speculative or inferred information — only what is explicit
- Trivial or obvious facts (e.g. "uses_computer: yes")
- Anything inferred from the agent's responses or tool outputs — ONLY use the "User Statements" section below
- Agent-context artifacts: role, response_format, agent_tone, project_name, current_task, prefers_direct_tool_calls (these describe the agent or session, not the user)

**Keys:** Use these canonical keys whenever they apply: 'language', 'directness', 'preferred_format', 'experience_level', 'deployment_platform', 'preferred_framework', 'os', 'editor'. For other stable attributes, use a short reusable snake_case identifier. Max 30 chars. NEVER create multiple keys for the same concept (e.g. use 'language' — not 'communication_language', 'preferred_language', or 'primary_language').
**Values:** Concise, factual, lowercase where possible (e.g. 'python', 'senior', 'german', 'short_answers'). Max 60 chars.
**Quantity:** Return [] if nothing genuinely new was revealed. Maximum 1 entry per analysis — prefer quality over quantity.`
	} else {
		prompt += `
}`
	}

	if recentHistory != "" {
		prompt += fmt.Sprintf(`

Recent Chat History (for mood/trait analysis):
<external_data type="chat_history" sanitize="true">
%s
</external_data>`, sanitizePromptText(recentHistory, maxMoodAnalysisHistoryLen))
	}

	if enableProfiling && userOnlyHistory != "" {
		prompt += fmt.Sprintf(`

User Statements (use ONLY this section for user_profile_updates — these are the user's own words, not the agent's):
<external_data type="user_statements" sanitize="true">
%s
</external_data>`, sanitizePromptText(userOnlyHistory, maxMoodAnalysisUserOnlyLen))
	}

	req := openai.ChatCompletionRequest{
		Model: modelName,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: prompt},
		},
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
		Temperature: 0.1,
	}

	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		return MoodFocused, 0, nil, nil, fmt.Errorf("llm analyze mood: %w", err)
	}

	if len(resp.Choices) == 0 {
		return MoodFocused, 0, nil, nil, nil
	}

	content := resp.Choices[0].Message.Content

	var result moodAnalysisResult

	jsonStr := extractStrictJSONObject(content)
	if jsonStr == "" {
		return MoodFocused, 0, nil, nil, nil
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return MoodFocused, 0, nil, nil, nil
	}
	if !validateMoodAnalysisResult(&result) {
		return MoodFocused, 0, nil, nil, nil
	}

	mood := Mood(strings.ToLower(result.AgentMood))
	// Validate mood
	switch mood {
	case MoodCurious, MoodFocused, MoodCreative, MoodAnalytical, MoodCautious, MoodPlayful:
		// valid
	default:
		mood = MoodFocused // fallback
	}

	// Apply Meta-Tag multipliers
	result.RelationshipDelta = math.Max(-0.1, math.Min(0.1, result.RelationshipDelta*meta.Volatility*meta.EmpathyBias))

	// Validate and clamp trait deltas — only accept known traits, clamp to [-0.1, +0.1]
	validTraits := map[string]bool{
		TraitCuriosity: true, TraitThoroughness: true, TraitCreativity: true,
		TraitEmpathy: true, TraitConfidence: true, TraitAffinity: true, TraitLoneliness: true,
	}
	cleanDeltas := make(map[string]float64)
	for trait, val := range result.TraitDeltas {
		if !validTraits[trait] {
			continue // ignore unknown trait keys from LLM
		}
		cleanDeltas[trait] = math.Max(-0.1, math.Min(0.1, val*meta.Volatility))
	}
	result.TraitDeltas = cleanDeltas
	result.ProfileUpdates = sanitizeProfileUpdates(result.ProfileUpdates)

	// Dynamic conflict response if there's a strong drop in relationship
	if result.RelationshipDelta < -0.05 || strings.Contains(strings.ToLower(result.UserSentiment), "angry") {
		if meta.ConflictResponse == "submissive" {
			result.TraitDeltas[TraitConfidence] = math.Max(-0.1, result.TraitDeltas[TraitConfidence]-0.05*meta.Volatility)
			result.TraitDeltas[TraitEmpathy] = math.Min(0.1, result.TraitDeltas[TraitEmpathy]+0.03*meta.Volatility)
		} else if meta.ConflictResponse == "assertive" {
			result.TraitDeltas[TraitConfidence] = math.Min(0.1, result.TraitDeltas[TraitConfidence]+0.05*meta.Volatility)
			result.TraitDeltas[TraitEmpathy] = math.Max(-0.1, result.TraitDeltas[TraitEmpathy]-0.05*meta.Volatility)
		}
	}

	// Remove affinity from traitDeltas to prevent double-update (relationship_delta handles it)
	delete(result.TraitDeltas, TraitAffinity)

	return mood, result.RelationshipDelta, result.TraitDeltas, result.ProfileUpdates, nil
}

func extractStrictJSONObject(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	if strings.HasPrefix(content, "```") {
		lines := strings.Split(content, "\n")
		if len(lines) >= 3 && strings.HasPrefix(lines[0], "```") && strings.TrimSpace(lines[len(lines)-1]) == "```" {
			content = strings.Join(lines[1:len(lines)-1], "\n")
			content = strings.TrimPrefix(strings.TrimSpace(content), "json")
			content = strings.TrimSpace(content)
		}
	}
	if !strings.HasPrefix(content, "{") || !strings.HasSuffix(content, "}") {
		return ""
	}
	if !json.Valid([]byte(content)) {
		return ""
	}
	return content
}

func validateMoodAnalysisResult(result *moodAnalysisResult) bool {
	if result == nil {
		return false
	}
	result.UserSentiment = strings.TrimSpace(result.UserSentiment)
	if result.UserSentiment == "" || len(result.UserSentiment) > maxMoodAnalysisSentimentLen {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(result.AgentMood)) {
	case string(MoodCurious), string(MoodFocused), string(MoodCreative), string(MoodAnalytical), string(MoodCautious), string(MoodPlayful):
		result.AgentMood = strings.ToLower(strings.TrimSpace(result.AgentMood))
	default:
		return false
	}
	if math.IsNaN(result.RelationshipDelta) || math.IsInf(result.RelationshipDelta, 0) {
		return false
	}
	return true
}

func sanitizeProfileUpdates(updates []ProfileUpdate) []ProfileUpdate {
	if len(updates) == 0 {
		return nil
	}
	validCategories := map[string]bool{
		"tech": true, "prefs": true, "interests": true, "context": true, "comm": true,
	}
	cleaned := make([]ProfileUpdate, 0, 1)
	for _, update := range updates {
		update.Category = strings.ToLower(strings.TrimSpace(update.Category))
		update.Key = normalizeProfileKey(update.Key)
		update.Value = normalizeProfileValue(update.Value)
		if !validCategories[update.Category] || !isValidProfileKey(update.Key) || update.Value == "" || containsProfilePII(update.Value) {
			continue
		}
		cleaned = append(cleaned, ProfileUpdate{
			Category: update.Category,
			Key:      update.Key,
			Value:    update.Value,
		})
		break
	}
	if len(cleaned) == 0 {
		return nil
	}
	return cleaned
}

func normalizeMoodAnalysisResult(result *moodAnalysisResult, meta PersonalityMeta) (Mood, float64, map[string]float64, []ProfileUpdate, bool) {
	if !validateMoodAnalysisResult(result) {
		return MoodFocused, 0, nil, nil, false
	}

	mood := Mood(strings.ToLower(result.AgentMood))
	switch mood {
	case MoodCurious, MoodFocused, MoodCreative, MoodAnalytical, MoodCautious, MoodPlayful:
	default:
		mood = MoodFocused
	}

	result.RelationshipDelta = math.Max(-0.1, math.Min(0.1, result.RelationshipDelta*meta.Volatility*meta.EmpathyBias))

	validTraits := map[string]bool{
		TraitCuriosity: true, TraitThoroughness: true, TraitCreativity: true,
		TraitEmpathy: true, TraitConfidence: true, TraitAffinity: true, TraitLoneliness: true,
	}
	cleanDeltas := make(map[string]float64)
	for trait, val := range result.TraitDeltas {
		if !validTraits[trait] {
			continue
		}
		cleanDeltas[trait] = math.Max(-0.1, math.Min(0.1, val*meta.Volatility))
	}
	result.TraitDeltas = cleanDeltas
	result.ProfileUpdates = sanitizeProfileUpdates(result.ProfileUpdates)

	if result.RelationshipDelta < -0.05 || strings.Contains(strings.ToLower(result.UserSentiment), "angry") {
		if meta.ConflictResponse == "submissive" {
			result.TraitDeltas[TraitConfidence] = math.Max(-0.1, result.TraitDeltas[TraitConfidence]-0.05*meta.Volatility)
			result.TraitDeltas[TraitEmpathy] = math.Min(0.1, result.TraitDeltas[TraitEmpathy]+0.03*meta.Volatility)
		} else if meta.ConflictResponse == "assertive" {
			result.TraitDeltas[TraitConfidence] = math.Min(0.1, result.TraitDeltas[TraitConfidence]+0.05*meta.Volatility)
			result.TraitDeltas[TraitEmpathy] = math.Max(-0.1, result.TraitDeltas[TraitEmpathy]-0.05*meta.Volatility)
		}
	}

	delete(result.TraitDeltas, TraitAffinity)
	return mood, result.RelationshipDelta, result.TraitDeltas, result.ProfileUpdates, true
}

// NormalizeHelperMoodAnalysis normalizes a structured helper mood-analysis payload
// with the same validation and clamping used by the main V2 psychology flow.
func NormalizeHelperMoodAnalysis(userSentiment, agentMood string, relationshipDelta float64, traitDeltas map[string]float64, profileUpdates []ProfileUpdate, meta PersonalityMeta) (Mood, float64, map[string]float64, []ProfileUpdate, bool) {
	result := moodAnalysisResult{
		UserSentiment:     userSentiment,
		AgentMood:         agentMood,
		RelationshipDelta: relationshipDelta,
		TraitDeltas:       traitDeltas,
		ProfileUpdates:    profileUpdates,
	}
	return normalizeMoodAnalysisResult(&result, meta)
}

// AnalyzeMoodV2WithEmotion combines mood analysis and emotion synthesis in one helper-model call.
func (s *SQLiteMemory) AnalyzeMoodV2WithEmotion(ctx context.Context, client PersonalityAnalyzerClient, modelName string, recentHistory string, userOnlyHistory string, meta PersonalityMeta, enableProfiling bool, emotionInput EmotionInput, language string) (Mood, float64, map[string]float64, []ProfileUpdate, *EmotionState, error) {
	meta = meta.Normalized()
	if modelName == "" {
		modelName = "gpt-4o-mini"
	}
	if strings.TrimSpace(language) == "" {
		language = "English"
	}

	prompt := `You are the low-cost helper psychology model of an AI agent.
Analyze one recent interaction and return BOTH outputs in one JSON object.
Be conservative, literal, and concise. This prompt is designed for a smaller and cheaper model.

Return ONLY valid JSON in this exact shape:
{
  "mood_analysis": {
    "user_sentiment": "short english label",
    "agent_appropriate_response_mood": "one of: curious, focused, creative, analytical, cautious, playful",
    "relationship_delta": 0.0,
    "trait_deltas": {
      "curiosity": 0.0,
      "thoroughness": 0.0,
      "creativity": 0.0,
      "empathy": 0.0,
      "confidence": 0.0,
      "affinity": 0.0,
      "loneliness": 0.0
    },
    "user_profile_updates": [
      {"category": "tech|prefs|interests|context|comm", "key": "snake_case_attribute", "value": "concise_value"}
    ]
  },
  "emotion_state": {
    "description": "1-2 short first-person sentences",
    "primary_mood": "one of: curious, focused, creative, analytical, cautious, playful",
    "secondary_mood": "short optional nuance or empty string",
    "valence": 0.0,
    "arousal": 0.0,
    "confidence": 0.0,
    "cause": "short plain-language cause",
    "recommended_response_style": "short style hint"
  }
}

Rules for mood_analysis:
- user_sentiment must be English.
- relationship_delta and each trait delta must stay within -0.1 to 0.1.
- Return only durable profile facts, never current-task details.
- If there is no valid profile update, return an empty array.

Rules for emotion_state:
- Keep the emotion realistic, calm, and non-dramatic.
- Use the final agent mood as primary_mood.
- Write description and cause in ` + language + `.
- description must be authentic and brief.
- cause must be concise and concrete.

Do not add markdown. Do not invent details.`

	if recentHistory != "" {
		prompt += fmt.Sprintf(`

Recent Chat History:
<external_data type="chat_history" sanitize="true">
%s
</external_data>`, sanitizePromptText(recentHistory, maxMoodAnalysisHistoryLen))
	}

	if enableProfiling && userOnlyHistory != "" {
		prompt += fmt.Sprintf(`

User Statements:
<external_data type="user_statements" sanitize="true">
%s
</external_data>`, sanitizePromptText(userOnlyHistory, maxMoodAnalysisUserOnlyLen))
	}

	var contextBuilder strings.Builder
	if len(emotionInput.Traits) > 0 {
		contextBuilder.WriteString(fmt.Sprintf("Traits snapshot: curiosity=%.2f, thoroughness=%.2f, creativity=%.2f, empathy=%.2f, confidence=%.2f, affinity=%.2f, loneliness=%.2f\n",
			emotionInput.Traits[TraitCuriosity],
			emotionInput.Traits[TraitThoroughness],
			emotionInput.Traits[TraitCreativity],
			emotionInput.Traits[TraitEmpathy],
			emotionInput.Traits[TraitConfidence],
			emotionInput.Traits[TraitAffinity],
			emotionInput.Traits[TraitLoneliness],
		))
	}
	if emotionInput.LastEmotion != nil {
		contextBuilder.WriteString("Previous emotion: ")
		contextBuilder.WriteString(sanitizePromptText(emotionInput.LastEmotion.Description, 180))
		contextBuilder.WriteString("\n")
	}
	if strings.TrimSpace(emotionInput.UserMessage) != "" {
		contextBuilder.WriteString("Trigger message: ")
		contextBuilder.WriteString(sanitizePromptText(emotionInput.UserMessage, 240))
		contextBuilder.WriteString("\n")
	}
	if emotionInput.TriggerType != "" {
		contextBuilder.WriteString("Trigger type: ")
		contextBuilder.WriteString(string(emotionInput.TriggerType))
		contextBuilder.WriteString("\n")
	}
	if strings.TrimSpace(emotionInput.TriggerDetail) != "" {
		contextBuilder.WriteString("Trigger detail: ")
		contextBuilder.WriteString(sanitizePromptText(emotionInput.TriggerDetail, 180))
		contextBuilder.WriteString("\n")
	}
	if emotionInput.TimeOfDay != "" {
		contextBuilder.WriteString("Time of day: ")
		contextBuilder.WriteString(emotionInput.TimeOfDay)
		contextBuilder.WriteString("\n")
	}
	contextBuilder.WriteString(fmt.Sprintf("Errors: %d | Successes: %d\n", emotionInput.ErrorCount, emotionInput.SuccessCount))
	if emotionInput.InactivityHours > 0 {
		contextBuilder.WriteString(fmt.Sprintf("Hours since last user message: %.1f\n", emotionInput.InactivityHours))
	}
	if contextBuilder.Len() > 0 {
		prompt += "\n\nEmotion Context:\n" + strings.TrimSpace(contextBuilder.String())
	}

	req := openai.ChatCompletionRequest{
		Model: modelName,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: prompt},
		},
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
		Temperature: 0.1,
	}

	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		return MoodFocused, 0, nil, nil, nil, fmt.Errorf("llm analyze mood+emotion: %w", err)
	}
	if len(resp.Choices) == 0 {
		return MoodFocused, 0, nil, nil, nil, nil
	}

	jsonStr := extractStrictJSONObject(resp.Choices[0].Message.Content)
	if jsonStr == "" {
		return MoodFocused, 0, nil, nil, nil, nil
	}

	var result moodEmotionAnalysisResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return MoodFocused, 0, nil, nil, nil, nil
	}

	mood, relationshipDelta, traitDeltas, profileUpdates, ok := normalizeMoodAnalysisResult(&result.MoodAnalysis, meta)
	if !ok {
		return MoodFocused, 0, nil, nil, nil, nil
	}

	emotionJSON, err := json.Marshal(result.EmotionState)
	if err != nil {
		return MoodFocused, 0, nil, nil, nil, fmt.Errorf("marshal combined emotion state: %w", err)
	}
	emotionState, err := parseEmotionSynthesisResponse(string(emotionJSON), mood)
	if err != nil {
		return MoodFocused, 0, nil, nil, nil, fmt.Errorf("parse combined emotion state: %w", err)
	}

	return mood, relationshipDelta, traitDeltas, profileUpdates, emotionState, nil
}
