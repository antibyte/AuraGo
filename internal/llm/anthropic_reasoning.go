package llm

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

// The SDK has one reasoning string. This versioned internal envelope preserves
// native signed blocks without adding fields to, or forking, the SDK.
const anthropicReasoningPrefix = "aurago:anthropic-reasoning:v1:"

func encodeAnthropicReasoning(blocks []json.RawMessage) string {
	if len(blocks) == 0 {
		return ""
	}
	data, _ := json.Marshal(blocks)
	return anthropicReasoningPrefix + base64.StdEncoding.EncodeToString(data)
}
func decodeAnthropicReasoning(value string) []json.RawMessage {
	if !strings.HasPrefix(value, anthropicReasoningPrefix) {
		return nil
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, anthropicReasoningPrefix))
	if err != nil {
		return nil
	}
	var blocks []json.RawMessage
	if json.Unmarshal(data, &blocks) != nil {
		return nil
	}
	for _, raw := range blocks {
		var block struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(raw, &block) != nil || (block.Type != "thinking" && block.Type != "redacted_thinking") {
			return nil
		}
	}
	return blocks
}

// ReasoningText returns displayable reasoning, excluding signatures and redacted data.
// Persist and send the original string to preserve continuation signatures.
func ReasoningText(value string) string {
	if !strings.HasPrefix(value, anthropicReasoningPrefix) {
		return value
	}
	var text strings.Builder
	for _, raw := range decodeAnthropicReasoning(value) {
		var block struct {
			Thinking string `json:"thinking"`
		}
		_ = json.Unmarshal(raw, &block)
		text.WriteString(block.Thinking)
	}
	return text.String()
}
func (b anthropicContentBlock) MarshalJSON() ([]byte, error) {
	if len(b.native) > 0 {
		return b.native, nil
	}
	type plain anthropicContentBlock
	return json.Marshal(plain(b))
}
func (b *anthropicResponseBlock) UnmarshalJSON(data []byte) error {
	type plain anthropicResponseBlock
	// Real API thinking is a string; accept old object fixtures for compatibility.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	thinking := fields["thinking"]
	delete(fields, "thinking")
	rest, _ := json.Marshal(fields)
	var decoded plain
	if err := json.Unmarshal(rest, &decoded); err != nil {
		return err
	}
	*b = anthropicResponseBlock(decoded)
	if len(thinking) > 0 {
		var text string
		if json.Unmarshal(thinking, &text) == nil {
			b.Thinking = &anthropicThinkingContent{Thinking: text}
		} else if err := json.Unmarshal(thinking, &b.Thinking); err != nil {
			return err
		}
	}
	b.native = append(json.RawMessage(nil), data...)
	return nil
}
func nativeThinkingBlock(b anthropicResponseBlock) json.RawMessage {
	if len(b.native) > 0 {
		return b.native
	}
	text := ""
	if b.Thinking != nil {
		text = b.Thinking.Thinking
	}
	data, _ := json.Marshal(map[string]string{"type": b.Type, "thinking": text})
	return data
}
func appendThinkingField(raw json.RawMessage, key, value string) json.RawMessage {
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	var previous string
	_ = json.Unmarshal(fields[key], &previous)
	fields[key], _ = json.Marshal(previous + value)
	data, _ := json.Marshal(fields)
	return data
}
func anthropicAdaptiveModel(model string) bool {
	lower := strings.ToLower(model)
	return strings.Contains(lower, "claude-opus-4-6") || strings.Contains(lower, "claude-opus-4.6") || strings.Contains(lower, "claude-sonnet-4-6") || strings.Contains(lower, "claude-sonnet-4.6") || anthropicFixedSamplingModel(lower)
}
func anthropicFixedSamplingModel(model string) bool {
	lower := strings.ToLower(model)
	for _, family := range []string{"claude-opus-4-7", "claude-opus-4.7", "claude-opus-4-8", "claude-opus-4.8", "claude-opus-5", "claude-sonnet-5", "claude-fable-5", "claude-mythos"} {
		if strings.Contains(lower, family) {
			return true
		}
	}
	return false
}
