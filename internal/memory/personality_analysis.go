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

// AnalyzeMoodV2 uses an LLM to asynchronously analyze the user's sentiment and intent from the recent chat history.
// It returns the determined agent mood, the affinity (relationship) delta, granular trait deltas, and optional user profile updates.
// userOnlyHistory contains only the user-role messages and is used exclusively for profile extraction to avoid
// incorrectly attributing agent actions or tool results to the user's profile.
func (s *SQLiteMemory) AnalyzeMoodV2(ctx context.Context, client PersonalityAnalyzerClient, modelName string, recentHistory string, userOnlyHistory string, meta PersonalityMeta, enableProfiling bool) (Mood, float64, map[string]float64, []ProfileUpdate, error) {
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

**Keys:** Short, reusable snake_case identifiers (e.g. 'primary_language', 'experience_level', 'preferred_format'). Max 30 chars.
**Values:** Concise, factual, lowercase where possible (e.g. 'python', 'senior', 'german', 'short_answers'). Max 60 chars.
**Quantity:** Return [] if nothing genuinely new was revealed. Maximum 2 entries per analysis — prefer quality over quantity.`
	} else {
		prompt += `
}`
	}

	prompt += `

Recent Chat History (for mood/trait analysis):
` + recentHistory

	if enableProfiling && userOnlyHistory != "" {
		prompt += `

User Statements (use ONLY this section for user_profile_updates — these are the user's own words, not the agent's):
` + userOnlyHistory
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

	var result struct {
		UserSentiment     string             `json:"user_sentiment"`
		AgentMood         string             `json:"agent_appropriate_response_mood"`
		RelationshipDelta float64            `json:"relationship_delta"`
		TraitDeltas       map[string]float64 `json:"trait_deltas"`
		ProfileUpdates    []ProfileUpdate    `json:"user_profile_updates"`
	}

	// Try to parse the JSON with high robustness (LLMs often preface with markers or timestamps)
	content = strings.TrimSpace(content)
	jsonStart := strings.Index(content, "{")
	if jsonStart == -1 {
		return MoodFocused, 0, nil, nil, nil
	}

	// Search for the longest VALID JSON starting at 'jsonStart'
	jsonStr := ""
	bStr := content[jsonStart:]
	for j := strings.LastIndex(bStr, "}"); j != -1; j = strings.LastIndex(bStr[:j], "}") {
		candidate := bStr[:j+1]
		var tmp struct {
			UserSentiment string `json:"user_sentiment"`
		}
		if json.Unmarshal([]byte(candidate), &tmp) == nil && tmp.UserSentiment != "" {
			jsonStr = candidate
			break
		}
	}

	if jsonStr == "" {
		return MoodFocused, 0, nil, nil, nil
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
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
