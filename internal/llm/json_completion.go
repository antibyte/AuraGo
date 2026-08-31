package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/sashabaranov/go-openai"
)

var (
	ErrJSONCompletionEmpty     = errors.New("json_completion_empty")
	ErrJSONCompletionTruncated = errors.New("json_completion_truncated")
	ErrJSONCompletionInvalid   = errors.New("json_completion_invalid")
)

var jsonThinkingBlock = regexp.MustCompile(`(?is)<(?:think|thinking)(?:\s[^>]*)?>.*?</(?:think|thinking)>`)

// JSONResponseFormat requests provider-side JSON mode only when catalog or
// configured capability metadata confirms support.
func JSONResponseFormat(supported bool) *openai.ChatCompletionResponseFormat {
	if !supported {
		return nil
	}
	return &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeJSONObject}
}

// JSONContentFromResponse validates completion state and returns normalized
// JSON. Empty, truncated and invalid responses are never considered usable.
func JSONContentFromResponse(resp openai.ChatCompletionResponse) (string, error) {
	if len(resp.Choices) == 0 {
		return "", ErrJSONCompletionEmpty
	}
	if strings.EqualFold(strings.TrimSpace(string(resp.Choices[0].FinishReason)), "length") {
		return "", ErrJSONCompletionTruncated
	}
	return NormalizeJSONContent(resp.Choices[0].Message.Content)
}

// NormalizeJSONContent tolerates thinking blocks and Markdown fences while
// still requiring exactly one valid JSON document.
func NormalizeJSONContent(content string) (string, error) {
	content = strings.TrimSpace(jsonThinkingBlock.ReplaceAllString(content, ""))
	if strings.HasPrefix(strings.ToLower(content), "<think") || strings.HasPrefix(strings.ToLower(content), "<thinking") {
		if start := firstJSONStart(content); start >= 0 {
			content = content[start:]
		}
	}
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```") {
		if newline := strings.IndexByte(content, '\n'); newline >= 0 {
			content = content[newline+1:]
		} else {
			content = strings.TrimPrefix(content, "```")
		}
	}
	content = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(content), "```"))
	if content == "" {
		return "", ErrJSONCompletionEmpty
	}
	if !json.Valid([]byte(content)) {
		start := firstJSONStart(content)
		end := lastJSONEnd(content)
		if start >= 0 && end > start {
			content = strings.TrimSpace(content[start : end+1])
		}
	}
	if !json.Valid([]byte(content)) {
		return "", fmt.Errorf("%w: response was not valid JSON", ErrJSONCompletionInvalid)
	}
	return content, nil
}

func firstJSONStart(content string) int {
	object := strings.IndexByte(content, '{')
	array := strings.IndexByte(content, '[')
	switch {
	case object < 0:
		return array
	case array < 0:
		return object
	case object < array:
		return object
	default:
		return array
	}
}

func lastJSONEnd(content string) int {
	object := strings.LastIndexByte(content, '}')
	array := strings.LastIndexByte(content, ']')
	if object > array {
		return object
	}
	return array
}
