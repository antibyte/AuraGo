package decisions

import (
	"encoding/json"
	"fmt"
	"math"
	"mime"
	"strings"
)

const (
	maxStateBytes      = 256 << 10
	maxQuestions       = 16
	maxOptions         = 62
	maxImages          = 8
	maxImageBytes      = 12 << 20
	maxTotalImageBytes = 24 << 20
	maxInstructions    = 16 << 10
	maxQuestionIDBytes = 128
	maxOptionIDBytes   = 128
)

// ValidateRequest rejects ambiguous candidates and oversized untrusted input
// before it reaches either a local process or Docker runtime.
func ValidateRequest(request Request) error {
	if len(request.Questions) == 0 || len(request.Questions) > maxQuestions {
		return fmt.Errorf("questions must contain between 1 and %d entries", maxQuestions)
	}
	state, err := json.Marshal(request.State)
	if err != nil {
		return fmt.Errorf("encode decision state: %w", err)
	}
	if len(state) > maxStateBytes {
		return fmt.Errorf("decision state exceeds %d bytes", maxStateBytes)
	}
	questionIDs := make(map[string]struct{}, len(request.Questions))
	for index, question := range request.Questions {
		if strings.TrimSpace(question.ID) == "" {
			return fmt.Errorf("question %d requires an id", index)
		}
		if len(question.ID) > maxQuestionIDBytes {
			return fmt.Errorf("question %d id exceeds %d bytes", index, maxQuestionIDBytes)
		}
		if question.Type != "choice" && question.Type != "noul" && question.Type != "score" {
			return fmt.Errorf("question %q has unsupported type %q", question.ID, question.Type)
		}
		if _, exists := questionIDs[question.ID]; exists {
			return fmt.Errorf("duplicate question id %q", question.ID)
		}
		questionIDs[question.ID] = struct{}{}
		if len(question.Instructions) > maxInstructions {
			return fmt.Errorf("question %q instructions exceed %d bytes", question.ID, maxInstructions)
		}
		if question.Type == "noul" {
			if len(question.Options) != 0 && len(question.Options) != 2 {
				return fmt.Errorf("question %q noul options must be omitted or contain exactly no and yes", question.ID)
			}
		} else if len(question.Options) < 2 || len(question.Options) > maxOptions {
			return fmt.Errorf("question %q must have between 2 and %d options", question.ID, maxOptions)
		}
		optionIDs := make(map[string]struct{}, len(question.Options))
		for optionIndex, option := range question.Options {
			if strings.TrimSpace(option.ID) == "" || strings.TrimSpace(option.Label) == "" {
				return fmt.Errorf("question %q option %d requires an id and label", question.ID, optionIndex)
			}
			if len(option.ID) > maxOptionIDBytes {
				return fmt.Errorf("question %q option %d id exceeds %d bytes", question.ID, optionIndex, maxOptionIDBytes)
			}
			if len(option.Label) > maxInstructions {
				return fmt.Errorf("question %q option %d label exceeds %d bytes", question.ID, optionIndex, maxInstructions)
			}
			if _, exists := optionIDs[option.ID]; exists {
				return fmt.Errorf("question %q has duplicate option id %q", question.ID, option.ID)
			}
			optionIDs[option.ID] = struct{}{}
			if option.Value != nil && (math.IsNaN(*option.Value) || math.IsInf(*option.Value, 0)) {
				return fmt.Errorf("question %q option %q has a non-finite value", question.ID, option.ID)
			}
			if question.Type == "choice" && option.Value != nil {
				return fmt.Errorf("question %q type %q does not accept numeric option values", question.ID, question.Type)
			}
		}
		switch question.Type {
		case "noul":
			if len(question.Options) == 2 {
				if _, no := optionIDs["no"]; !no {
					return fmt.Errorf("question %q noul options must use ids no and yes", question.ID)
				}
				if _, yes := optionIDs["yes"]; !yes {
					return fmt.Errorf("question %q noul options must use ids no and yes", question.ID)
				}
				for _, option := range question.Options {
					if option.ID == "no" && option.Value != nil && *option.Value != 0 {
						return fmt.Errorf("question %q noul no option must have value zero", question.ID)
					}
					if option.ID == "yes" && option.Value != nil && *option.Value != 1 {
						return fmt.Errorf("question %q noul yes option must have value one", question.ID)
					}
				}
			}
		case "score":
			effectiveValues := make(map[float64]string, len(question.Options))
			for optionIndex, option := range question.Options {
				value := float64(optionIndex)
				if option.Value != nil {
					value = *option.Value
				}
				if prior, exists := effectiveValues[value]; exists {
					return fmt.Errorf("question %q options %q and %q have the same score value", question.ID, prior, option.ID)
				}
				effectiveValues[value] = option.ID
			}
		}
	}
	if len(request.Images) > maxImages {
		return fmt.Errorf("at most %d images are allowed", maxImages)
	}
	totalImageBytes := 0
	for index, image := range request.Images {
		if len(image.Data) == 0 || len(image.Data) > maxImageBytes {
			return fmt.Errorf("image %d must contain between 1 and %d bytes", index, maxImageBytes)
		}
		totalImageBytes += len(image.Data)
		if totalImageBytes > maxTotalImageBytes {
			return fmt.Errorf("images exceed %d total bytes", maxTotalImageBytes)
		}
		mediaType, _, err := mime.ParseMediaType(image.MIME)
		if err != nil || !supportedImageMIME(strings.ToLower(mediaType)) {
			return fmt.Errorf("image %d has an unsupported MIME type", index)
		}
	}
	return nil
}

func supportedImageMIME(mediaType string) bool {
	switch mediaType {
	case "image/jpeg", "image/png", "image/gif", "image/bmp":
		return true
	default:
		return false
	}
}

// NormalizeLogits applies a stable softmax to one logit per candidate.
func NormalizeLogits(logits []float64) ([]float64, error) {
	if len(logits) < 2 {
		return nil, fmt.Errorf("at least two candidate logits are required")
	}
	maxLogit := math.Inf(-1)
	for _, value := range logits {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, fmt.Errorf("candidate logits must be finite")
		}
		if value > maxLogit {
			maxLogit = value
		}
	}
	probabilities := make([]float64, len(logits))
	var total float64
	for index, value := range logits {
		probabilities[index] = math.Exp(value - maxLogit)
		total += probabilities[index]
	}
	if total == 0 || math.IsInf(total, 0) || math.IsNaN(total) {
		return nil, fmt.Errorf("candidate logits could not be normalized")
	}
	for index := range probabilities {
		probabilities[index] /= total
	}
	return probabilities, nil
}
