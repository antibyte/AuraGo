package decisions

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

const candidateAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

type runtimeCandidate struct {
	Code      string `json:"code"`
	TokenText string `json:"token_text"`
}

type scoredCandidate struct {
	Code    string  `json:"code"`
	TokenID int     `json:"token_id"`
	Logit   float64 `json:"logit"`
}

func optionsForQuestion(question Question) []Option {
	if question.Type != "noul" || len(question.Options) != 0 {
		return append([]Option(nil), question.Options...)
	}
	zero, one := 0.0, 1.0
	return []Option{
		{ID: "no", Label: "No", Value: &zero},
		{ID: "yes", Label: "Yes", Value: &one},
	}
}

func candidatesForOptions(options []Option) ([]runtimeCandidate, error) {
	if len(options) < 2 || len(options) > len(candidateAlphabet) {
		return nil, fmt.Errorf("candidate count is outside the supported range")
	}
	candidates := make([]runtimeCandidate, len(options))
	for index, option := range options {
		candidates[index] = runtimeCandidate{Code: option.ID, TokenText: candidateAlphabet[index : index+1]}
	}
	return candidates, nil
}

func buildPrompt(state any, question Question, options []Option, imageMarker string, imageCount int) (string, error) {
	stateText, isText := state.(string)
	if !isText {
		stateJSON, err := json.Marshal(state)
		if err != nil {
			return "", fmt.Errorf("encode state: %w", err)
		}
		stateText = string(stateJSON)
	}
	candidates, err := candidatesForOptions(options)
	if err != nil {
		return "", err
	}
	var prompt strings.Builder
	task := strings.TrimSpace(question.Instructions)
	if task == "" {
		task = question.Type
	}
	task = strings.Join(strings.Fields(task), " ")
	prompt.WriteString("Task: ")
	prompt.WriteString(task)
	prompt.WriteString("\nState: ")
	prompt.WriteString(stateText)
	prompt.WriteByte('\n')
	if imageCount > 0 {
		if imageMarker == "" || strings.ContainsAny(imageMarker, "\r\n") {
			return "", fmt.Errorf("runtime did not provide a valid image marker")
		}
		for index := 0; index < imageCount; index++ {
			prompt.WriteString(imageMarker)
			prompt.WriteByte('\n')
		}
	}
	prompt.WriteString("Options: [")
	for index, option := range options {
		if index > 0 {
			prompt.WriteString(", ")
		}
		prompt.WriteString(candidates[index].TokenText)
		prompt.WriteString(": ")
		prompt.WriteString(strings.Join(strings.Fields(option.Label), " "))
	}
	prompt.WriteString("]\n### Decision:\n")
	return prompt.String(), nil
}

func answerFromScores(question Question, options []Option, candidates []runtimeCandidate, scores []scoredCandidate) (Answer, error) {
	if len(candidates) != len(options) || len(scores) != len(candidates) {
		return Answer{}, fmt.Errorf("runtime returned a different number of candidate scores")
	}
	optionByCode := make(map[string]Option, len(options))
	for index, candidate := range candidates {
		if candidate.Code != options[index].ID {
			return Answer{}, fmt.Errorf("candidate mapping changed while scoring")
		}
		optionByCode[candidate.Code] = options[index]
	}
	logits := make([]float64, len(candidates))
	seen := make(map[string]struct{}, len(scores))
	seenTokenIDs := make(map[int]struct{}, len(scores))
	for _, score := range scores {
		if _, exists := optionByCode[score.Code]; !exists {
			return Answer{}, fmt.Errorf("runtime returned an unknown candidate code")
		}
		if _, exists := seen[score.Code]; exists {
			return Answer{}, fmt.Errorf("runtime returned a duplicate candidate score")
		}
		seen[score.Code] = struct{}{}
		if _, exists := seenTokenIDs[score.TokenID]; exists {
			return Answer{}, fmt.Errorf("runtime returned duplicate candidate token ids")
		}
		seenTokenIDs[score.TokenID] = struct{}{}
		if math.IsNaN(score.Logit) || math.IsInf(score.Logit, 0) || score.TokenID < 0 {
			return Answer{}, fmt.Errorf("runtime returned an invalid candidate score")
		}
	}
	for index, candidate := range candidates {
		var found bool
		for _, score := range scores {
			if score.Code == candidate.Code {
				logits[index] = score.Logit
				found = true
				break
			}
		}
		if !found {
			return Answer{}, fmt.Errorf("runtime omitted a candidate score")
		}
	}
	values, err := NormalizeLogits(logits)
	if err != nil {
		return Answer{}, err
	}
	probabilities := make(map[string]float64, len(options))
	selectedIndex := 0
	for index, option := range options {
		probabilities[option.ID] = values[index]
		if values[index] > values[selectedIndex] {
			selectedIndex = index
		}
	}
	selected := options[selectedIndex]
	answer := Answer{
		Type:          question.Type,
		Decision:      selected.ID,
		Probabilities: probabilities,
		Confidence:    values[selectedIndex],
	}
	switch question.Type {
	case "choice":
		answer.Choice = selected.Label
	case "score":
		expected := 0.0
		for index, option := range options {
			value := float64(index)
			if option.Value != nil {
				value = *option.Value
			}
			expected += values[index] * value
		}
		if math.IsNaN(expected) || math.IsInf(expected, 0) {
			return Answer{}, fmt.Errorf("expected score is not finite")
		}
		answer.Score = &expected
	case "noul":
		yes, ok := probabilities["yes"]
		if !ok {
			return Answer{}, fmt.Errorf("noul question has no yes candidate")
		}
		answer.Noul = &yes
	default:
		return Answer{}, fmt.Errorf("unsupported question type %q", question.Type)
	}
	return answer, nil
}
