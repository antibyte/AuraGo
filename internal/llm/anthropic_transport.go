package llm

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const anthropicAPIVersion = "2024-10-22"
const anthropicDefaultMaxTokens = 8192

// anthropicTransport is an http.RoundTripper that translates OpenAI-format
// chat completion requests into the Anthropic Messages API format and maps
// responses back. This allows the go-openai client to talk directly to
// api.anthropic.com without any changes to the agent loop, tool dispatch,
// or streaming assembly.
//
// Pattern: same as miniMaxTransport in client.go — intercept in-flight,
// translate bidirectionally, zero blast radius.
type anthropicTransport struct {
	base http.RoundTripper
}

// ---------------------------------------------------------------------------
// Anthropic API types (request)
// ---------------------------------------------------------------------------

type anthropicRequest struct {
	Model         string               `json:"model"`
	MaxTokens     int                  `json:"max_tokens"`
	System        string               `json:"system,omitempty"`
	Messages      []anthropicMessage   `json:"messages"`
	Stream        bool                 `json:"stream,omitempty"`
	Temperature   *float32             `json:"temperature,omitempty"`
	TopP          *float32             `json:"top_p,omitempty"`
	StopSequences []string             `json:"stop_sequences,omitempty"`
	Tools         []anthropicToolDef   `json:"tools,omitempty"`
	ToolChoice    *anthropicToolChoice `json:"tool_choice,omitempty"`
	Metadata      *anthropicMetadata   `json:"metadata,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or []anthropicContentBlock
}

type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`

	// tool_use
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// tool_result
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   any    `json:"content,omitempty"` // string or []anthropicContentBlock
	IsError   *bool  `json:"is_error,omitempty"`

	// image
	Source *anthropicImageSource `json:"source,omitempty"`
}

type anthropicImageSource struct {
	Type      string `json:"type"`                 // "base64" or "url"
	MediaType string `json:"media_type,omitempty"` // "image/png", "image/jpeg", etc.
	Data      string `json:"data,omitempty"`       // base64-encoded bytes (type="base64")
	URL       string `json:"url,omitempty"`        // external URL (type="url")
}

type anthropicToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type anthropicToolChoice struct {
	Type string `json:"type"`           // "auto", "any", "none", "tool"
	Name string `json:"name,omitempty"` // for type="tool"
}

type anthropicMetadata struct {
	UserID string `json:"user_id,omitempty"`
}

// ---------------------------------------------------------------------------
// Anthropic API types (response)
// ---------------------------------------------------------------------------

type anthropicResponse struct {
	ID           string                   `json:"id"`
	Type         string                   `json:"type"`
	Role         string                   `json:"role"`
	Content      []anthropicResponseBlock `json:"content"`
	Model        string                   `json:"model"`
	StopReason   string                   `json:"stop_reason"`
	StopSequence *string                  `json:"stop_sequence"`
	Usage        anthropicUsage           `json:"usage"`
}

type anthropicResponseBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ---------------------------------------------------------------------------
// Anthropic SSE event types (streaming)
// ---------------------------------------------------------------------------

type anthropicStreamMessageStart struct {
	Type    string            `json:"type"`
	Message anthropicResponse `json:"message"`
}

type anthropicStreamContentBlockStart struct {
	Type         string                 `json:"type"`
	Index        int                    `json:"index"`
	ContentBlock anthropicResponseBlock `json:"content_block"`
}

type anthropicStreamContentBlockDelta struct {
	Type  string                   `json:"type"`
	Index int                      `json:"index"`
	Delta anthropicStreamDeltaBody `json:"delta"`
}

type anthropicStreamDeltaBody struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
}

type anthropicStreamMessageDelta struct {
	Type  string `json:"type"`
	Delta struct {
		StopReason   string  `json:"stop_reason"`
		StopSequence *string `json:"stop_sequence"`
	} `json:"delta"`
	Usage *anthropicUsage `json:"usage,omitempty"`
}

type anthropicStreamError struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// ---------------------------------------------------------------------------
// OpenAI types used for serialization (request-side)
// ---------------------------------------------------------------------------

type openaiRequest struct {
	Model            string            `json:"model"`
	Messages         []json.RawMessage `json:"messages"`
	Stream           bool              `json:"stream"`
	Temperature      *float32          `json:"temperature,omitempty"`
	TopP             *float32          `json:"top_p,omitempty"`
	MaxTokens        int               `json:"max_tokens,omitempty"`
	Stop             any               `json:"stop,omitempty"`
	Tools            []json.RawMessage `json:"tools,omitempty"`
	ToolChoice       any               `json:"tool_choice,omitempty"`
	User             string            `json:"user,omitempty"`
	FrequencyPenalty float32           `json:"frequency_penalty,omitempty"`
	PresencePenalty  float32           `json:"presence_penalty,omitempty"`
}

type openaiMessage struct {
	Role       string           `json:"role"`
	Content    any              `json:"content"` // string or []openaiContentPart
	ToolCalls  []openaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	Name       string           `json:"name,omitempty"`
}

type openaiContentPart struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *openaiImageURL `json:"image_url,omitempty"`
}

type openaiImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type openaiToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function openaiToolCallFn `json:"function"`
	Index    *int             `json:"index,omitempty"`
}

type openaiToolCallFn struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openaiToolDef struct {
	Type     string         `json:"type"`
	Function openaiFunction `json:"function"`
}

type openaiFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// ---------------------------------------------------------------------------
// OpenAI types used for serialization (response-side)
// ---------------------------------------------------------------------------

type openaiResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []openaiChoice `json:"choices"`
	Usage   openaiUsage    `json:"usage"`
}

type openaiChoice struct {
	Index        int               `json:"index"`
	Message      openaiRespMessage `json:"message,omitempty"`
	Delta        *openaiRespDelta  `json:"delta,omitempty"`
	FinishReason *string           `json:"finish_reason"`
}

type openaiRespMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []openaiToolCall `json:"tool_calls,omitempty"`
}

type openaiRespDelta struct {
	Role      string           `json:"role,omitempty"`
	Content   string           `json:"content,omitempty"`
	ToolCalls []openaiToolCall `json:"tool_calls,omitempty"`
}

type openaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openaiStreamChunk struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []openaiChoice `json:"choices"`
	Usage   *openaiUsage   `json:"usage,omitempty"`
}

// ---------------------------------------------------------------------------
// RoundTrip — main entry point
// ---------------------------------------------------------------------------

func (t *anthropicTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Only intercept chat completion requests; pass everything else through.
	if req.Body == nil || req.Method != http.MethodPost ||
		!strings.HasSuffix(req.URL.Path, "/chat/completions") {
		return t.baseTransport().RoundTrip(req)
	}

	body, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("anthropic transport: read body: %w", err)
	}

	var oaiReq openaiRequest
	if err := json.Unmarshal(body, &oaiReq); err != nil {
		return nil, fmt.Errorf("anthropic transport: unmarshal request: %w", err)
	}

	antReq, err := translateOpenAIToAnthropic(oaiReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic transport: translate request: %w", err)
	}

	antBody, err := json.Marshal(antReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic transport: marshal anthropic request: %w", err)
	}

	// Rewrite the request for the Anthropic API.
	clone := req.Clone(req.Context())
	clone.URL.Path = strings.Replace(clone.URL.Path, "/chat/completions", "/messages", 1)
	clone.Body = io.NopCloser(bytes.NewReader(antBody))
	clone.ContentLength = int64(len(antBody))
	clone.Header = req.Header.Clone()

	// Auth: replace Bearer token with x-api-key header.
	if auth := clone.Header.Get("Authorization"); auth != "" {
		apiKey := strings.TrimPrefix(auth, "Bearer ")
		clone.Header.Del("Authorization")
		clone.Header.Set("x-api-key", apiKey)
	}
	clone.Header.Set("anthropic-version", anthropicAPIVersion)
	clone.Header.Set("Content-Type", "application/json")

	resp, err := t.baseTransport().RoundTrip(clone)
	if err != nil {
		return nil, err
	}

	// Error responses: wrap in OpenAI-style error JSON.
	if resp.StatusCode >= 400 {
		return translateAnthropicError(resp)
	}

	if oaiReq.Stream {
		return translateAnthropicStream(resp, oaiReq.Model)
	}
	return translateAnthropicResponse(resp)
}

func (t *anthropicTransport) baseTransport() http.RoundTripper {
	if t.base != nil {
		return t.base
	}
	return http.DefaultTransport
}

// ---------------------------------------------------------------------------
// Request translation: OpenAI → Anthropic
// ---------------------------------------------------------------------------

func translateOpenAIToAnthropic(oai openaiRequest) (anthropicRequest, error) {
	ant := anthropicRequest{
		Model:       oai.Model,
		Stream:      oai.Stream,
		Temperature: oai.Temperature,
		TopP:        oai.TopP,
	}

	// max_tokens is required by Anthropic. Default to 8192 if unset.
	if oai.MaxTokens > 0 {
		ant.MaxTokens = oai.MaxTokens
	} else {
		ant.MaxTokens = anthropicDefaultMaxTokens
	}

	// stop → stop_sequences (Anthropic only accepts an array)
	ant.StopSequences = translateStopSequences(oai.Stop)

	// tools
	if len(oai.Tools) > 0 {
		tools, err := translateTools(oai.Tools)
		if err != nil {
			return ant, fmt.Errorf("translate tools: %w", err)
		}
		ant.Tools = tools
	}

	// tool_choice
	if oai.ToolChoice != nil {
		tc, err := translateToolChoice(oai.ToolChoice)
		if err != nil {
			return ant, fmt.Errorf("translate tool_choice: %w", err)
		}
		ant.ToolChoice = tc
	}

	// user → metadata.user_id
	if oai.User != "" {
		ant.Metadata = &anthropicMetadata{UserID: oai.User}
	}

	// messages: extract system messages, convert tool results, merge consecutive users
	system, messages, err := translateMessages(oai.Messages)
	if err != nil {
		return ant, fmt.Errorf("translate messages: %w", err)
	}
	ant.System = system
	ant.Messages = messages

	return ant, nil
}

func translateStopSequences(stop any) []string {
	if stop == nil {
		return nil
	}
	switch v := stop.(type) {
	case string:
		if v != "" {
			return []string{v}
		}
	case []interface{}:
		var seqs []string
		for _, s := range v {
			if str, ok := s.(string); ok && str != "" {
				seqs = append(seqs, str)
			}
		}
		return seqs
	}
	return nil
}

func translateTools(rawTools []json.RawMessage) ([]anthropicToolDef, error) {
	var result []anthropicToolDef
	for _, raw := range rawTools {
		var oaiTool openaiToolDef
		if err := json.Unmarshal(raw, &oaiTool); err != nil {
			return nil, err
		}
		result = append(result, anthropicToolDef{
			Name:        oaiTool.Function.Name,
			Description: oaiTool.Function.Description,
			InputSchema: oaiTool.Function.Parameters,
		})
	}
	return result, nil
}

func translateToolChoice(tc any) (*anthropicToolChoice, error) {
	switch v := tc.(type) {
	case string:
		switch v {
		case "auto":
			return &anthropicToolChoice{Type: "auto"}, nil
		case "none":
			// Anthropic doesn't have "none" directly — omit tools from request.
			// But as a tool_choice value, we can't actually send it. Return nil
			// and let the caller strip tools if needed.
			return nil, nil
		case "required":
			return &anthropicToolChoice{Type: "any"}, nil
		}
		return &anthropicToolChoice{Type: "auto"}, nil
	case map[string]interface{}:
		// OpenAI specific tool choice: {"type":"function","function":{"name":"X"}}
		if fnIface, ok := v["function"]; ok {
			if fnMap, ok := fnIface.(map[string]interface{}); ok {
				if name, ok := fnMap["name"].(string); ok {
					return &anthropicToolChoice{Type: "tool", Name: name}, nil
				}
			}
		}
		// Fallback: try type field directly
		if typ, ok := v["type"].(string); ok {
			return &anthropicToolChoice{Type: typ}, nil
		}
		return &anthropicToolChoice{Type: "auto"}, nil
	}
	return &anthropicToolChoice{Type: "auto"}, nil
}

func translateMessages(rawMsgs []json.RawMessage) (string, []anthropicMessage, error) {
	var systemParts []string
	var antMsgs []anthropicMessage

	for _, raw := range rawMsgs {
		var msg openaiMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			return "", nil, fmt.Errorf("unmarshal message: %w", err)
		}

		switch msg.Role {
		case "system":
			text := extractTextContent(msg.Content)
			if text != "" {
				systemParts = append(systemParts, text)
			}

		case "assistant":
			if len(msg.ToolCalls) > 0 {
				// Assistant message with tool calls → Anthropic content blocks
				blocks := buildAssistantBlocks(msg)
				antMsgs = appendMergedMessage("assistant", blocks, antMsgs)
			} else {
				text := extractTextContent(msg.Content)
				if text == "" {
					text = " " // Anthropic requires non-empty content
				}
				antMsgs = appendMergedMessage("assistant", text, antMsgs)
			}

		case "tool":
			// OpenAI role="tool" → Anthropic role="user" with tool_result block
			block := anthropicContentBlock{
				Type:      "tool_result",
				ToolUseID: msg.ToolCallID,
				Content:   extractTextContent(msg.Content),
			}
			// Must merge into a user message
			antMsgs = appendToolResultBlock(block, antMsgs)

		case "user":
			content, err := translateUserContent(msg.Content)
			if err != nil {
				return "", nil, err
			}
			antMsgs = appendMergedMessage("user", content, antMsgs)
		}
	}

	// Ensure conversation starts with "user" role (Anthropic requirement)
	if len(antMsgs) > 0 && antMsgs[0].Role == "assistant" {
		antMsgs = append([]anthropicMessage{{Role: "user", Content: " "}}, antMsgs...)
	}

	return strings.Join(systemParts, "\n\n"), antMsgs, nil
}

// extractTextContent gets plain text from either a string or an array of content parts.
func extractTextContent(content any) string {
	if content == nil {
		return ""
	}
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		var parts []string
		for _, part := range v {
			if m, ok := part.(map[string]interface{}); ok {
				if m["type"] == "text" {
					if text, ok := m["text"].(string); ok {
						parts = append(parts, text)
					}
				}
			}
		}
		return strings.Join(parts, "\n")
	}
	return fmt.Sprintf("%v", content)
}

func buildAssistantBlocks(msg openaiMessage) []anthropicContentBlock {
	var blocks []anthropicContentBlock

	// Add text content first if present
	text := extractTextContent(msg.Content)
	if text != "" {
		blocks = append(blocks, anthropicContentBlock{Type: "text", Text: text})
	}

	// Convert tool calls to tool_use blocks
	for _, tc := range msg.ToolCalls {
		block := anthropicContentBlock{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: json.RawMessage(tc.Function.Arguments),
		}
		if len(block.Input) == 0 {
			block.Input = json.RawMessage("{}")
		}
		blocks = append(blocks, block)
	}

	return blocks
}

// appendMergedMessage appends a message, merging consecutive same-role messages.
// Anthropic forbids consecutive messages of the same role.
func appendMergedMessage(role string, content any, msgs []anthropicMessage) []anthropicMessage {
	if len(msgs) > 0 && msgs[len(msgs)-1].Role == role {
		// Merge into the last message
		last := &msgs[len(msgs)-1]
		last.Content = mergeContent(last.Content, content)
		return msgs
	}
	return append(msgs, anthropicMessage{Role: role, Content: content})
}

// appendToolResultBlock appends a tool_result block as a user message,
// merging with an existing trailing user message if present.
func appendToolResultBlock(block anthropicContentBlock, msgs []anthropicMessage) []anthropicMessage {
	if len(msgs) > 0 && msgs[len(msgs)-1].Role == "user" {
		last := &msgs[len(msgs)-1]
		last.Content = appendToContentBlocks(last.Content, block)
		return msgs
	}
	return append(msgs, anthropicMessage{
		Role:    "user",
		Content: []anthropicContentBlock{block},
	})
}

// mergeContent combines two content values (string or []anthropicContentBlock).
func mergeContent(existing, new any) any {
	existingBlocks := toContentBlocks(existing)
	newBlocks := toContentBlocks(new)
	return append(existingBlocks, newBlocks...)
}

func appendToContentBlocks(existing any, block anthropicContentBlock) []anthropicContentBlock {
	blocks := toContentBlocks(existing)
	return append(blocks, block)
}

func toContentBlocks(content any) []anthropicContentBlock {
	switch v := content.(type) {
	case string:
		if v == "" || v == " " {
			return nil
		}
		return []anthropicContentBlock{{Type: "text", Text: v}}
	case []anthropicContentBlock:
		return v
	default:
		return nil
	}
}

func translateUserContent(content any) (any, error) {
	if content == nil {
		return " ", nil // Anthropic requires non-empty content
	}

	switch v := content.(type) {
	case string:
		if v == "" {
			return " ", nil
		}
		return v, nil

	case []interface{}:
		var blocks []anthropicContentBlock
		for _, part := range v {
			m, ok := part.(map[string]interface{})
			if !ok {
				continue
			}
			partType, _ := m["type"].(string)
			switch partType {
			case "text":
				text, _ := m["text"].(string)
				if text != "" {
					blocks = append(blocks, anthropicContentBlock{Type: "text", Text: text})
				}
			case "image_url":
				imgBlock, err := translateImageURL(m)
				if err != nil {
					return nil, err
				}
				if imgBlock != nil {
					blocks = append(blocks, *imgBlock)
				}
			}
		}
		if len(blocks) == 0 {
			return " ", nil
		}
		return blocks, nil
	}

	return fmt.Sprintf("%v", content), nil
}

func translateImageURL(part map[string]interface{}) (*anthropicContentBlock, error) {
	imgURLIface, ok := part["image_url"]
	if !ok {
		return nil, nil
	}
	imgURLMap, ok := imgURLIface.(map[string]interface{})
	if !ok {
		return nil, nil
	}
	urlStr, _ := imgURLMap["url"].(string)
	if urlStr == "" {
		return nil, nil
	}

	// Handle data: URIs (base64 encoded images)
	if strings.HasPrefix(urlStr, "data:") {
		mediaType, data, err := parseDataURI(urlStr)
		if err != nil {
			return nil, fmt.Errorf("parse image data URI: %w", err)
		}
		return &anthropicContentBlock{
			Type: "image",
			Source: &anthropicImageSource{
				Type:      "base64",
				MediaType: mediaType,
				Data:      data,
			},
		}, nil
	}

	// External URLs: Anthropic supports URL sources since 2024-10-22.
	// The url-type source uses the "url" field, not "data".
	return &anthropicContentBlock{
		Type: "image",
		Source: &anthropicImageSource{
			Type: "url",
			URL:  urlStr,
		},
	}, nil
}

func parseDataURI(dataURI string) (mediaType, data string, err error) {
	// Format: data:image/png;base64,iVBOR...
	rest := strings.TrimPrefix(dataURI, "data:")
	parts := strings.SplitN(rest, ",", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid data URI format")
	}
	meta := parts[0] // e.g. "image/png;base64"
	data = parts[1]

	// Validate base64 data
	if _, err := base64.StdEncoding.DecodeString(data); err != nil {
		// Try URL-safe encoding
		if _, err := base64.URLEncoding.DecodeString(data); err != nil {
			return "", "", fmt.Errorf("invalid base64 data: %w", err)
		}
	}

	// Extract MIME type
	mediaType = strings.Split(meta, ";")[0]
	if mediaType == "" {
		mediaType = "image/png"
	}
	return mediaType, data, nil
}

func guessMediaType(url string) string {
	lower := strings.ToLower(url)
	// Strip query string and fragment before inspecting the file extension.
	if idx := strings.IndexByte(lower, '?'); idx != -1 {
		lower = lower[:idx]
	}
	if idx := strings.IndexByte(lower, '#'); idx != -1 {
		lower = lower[:idx]
	}
	switch {
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".gif"):
		return "image/gif"
	case strings.HasSuffix(lower, ".webp"):
		return "image/webp"
	default:
		return "image/jpeg"
	}
}

// ---------------------------------------------------------------------------
// Response translation: Anthropic → OpenAI (non-streaming)
// ---------------------------------------------------------------------------

func translateAnthropicResponse(resp *http.Response) (*http.Response, error) {
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("anthropic transport: read response: %w", err)
	}

	var antResp anthropicResponse
	if err := json.Unmarshal(body, &antResp); err != nil {
		return nil, fmt.Errorf("anthropic transport: unmarshal response: %w", err)
	}

	oaiResp := mapAnthropicToOpenAI(antResp)

	oaiBody, err := json.Marshal(oaiResp)
	if err != nil {
		return nil, fmt.Errorf("anthropic transport: marshal openai response: %w", err)
	}

	resp.Body = io.NopCloser(bytes.NewReader(oaiBody))
	resp.ContentLength = int64(len(oaiBody))
	resp.Header.Set("Content-Type", "application/json")
	return resp, nil
}

func mapAnthropicToOpenAI(ant anthropicResponse) openaiResponse {
	var content string
	var toolCalls []openaiToolCall
	toolIdx := 0

	for _, block := range ant.Content {
		switch block.Type {
		case "text":
			if content != "" {
				content += "\n"
			}
			content += block.Text
		case "tool_use":
			args := string(block.Input)
			if args == "" {
				args = "{}"
			}
			idx := toolIdx
			toolCalls = append(toolCalls, openaiToolCall{
				ID:   block.ID,
				Type: "function",
				Function: openaiToolCallFn{
					Name:      block.Name,
					Arguments: args,
				},
				Index: &idx,
			})
			toolIdx++
		}
	}

	finishReason := mapStopReason(ant.StopReason)

	return openaiResponse{
		ID:      "chatcmpl-" + strings.TrimPrefix(ant.ID, "msg_"),
		Object:  "chat.completion",
		Created: 0, // Anthropic doesn't return a timestamp
		Model:   ant.Model,
		Choices: []openaiChoice{
			{
				Index: 0,
				Message: openaiRespMessage{
					Role:      "assistant",
					Content:   content,
					ToolCalls: toolCalls,
				},
				FinishReason: &finishReason,
			},
		},
		Usage: openaiUsage{
			PromptTokens:     ant.Usage.InputTokens,
			CompletionTokens: ant.Usage.OutputTokens,
			TotalTokens:      ant.Usage.InputTokens + ant.Usage.OutputTokens,
		},
	}
}

func mapStopReason(reason string) string {
	switch reason {
	case "end_turn":
		return "stop"
	case "tool_use":
		return "tool_calls"
	case "max_tokens":
		return "length"
	case "stop_sequence":
		return "stop"
	default:
		return "stop"
	}
}

// ---------------------------------------------------------------------------
// Error translation: Anthropic error → OpenAI-style error response
// ---------------------------------------------------------------------------

func translateAnthropicError(resp *http.Response) (*http.Response, error) {
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("anthropic transport: read error response: %w", err)
	}

	// Try to parse Anthropic error format
	var antErr struct {
		Type  string `json:"type"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}

	errMsg := string(body)
	if json.Unmarshal(body, &antErr) == nil && antErr.Error.Message != "" {
		errMsg = antErr.Error.Message
	}

	// Map to OpenAI error format
	oaiErr := map[string]interface{}{
		"error": map[string]interface{}{
			"message": errMsg,
			"type":    antErr.Error.Type,
			"code":    strconv.Itoa(resp.StatusCode),
		},
	}

	oaiBody, _ := json.Marshal(oaiErr)
	resp.Body = io.NopCloser(bytes.NewReader(oaiBody))
	resp.ContentLength = int64(len(oaiBody))
	resp.Header.Set("Content-Type", "application/json")
	return resp, nil
}

// ---------------------------------------------------------------------------
// Streaming translation: Anthropic SSE → OpenAI SSE
// ---------------------------------------------------------------------------

func translateAnthropicStream(resp *http.Response, model string) (*http.Response, error) {
	pr, pw := io.Pipe()
	originalBody := resp.Body // capture before replacing

	go func() {
		defer pw.Close()
		translateStreamEvents(originalBody, pw, model)
		originalBody.Close()
	}()

	resp.Body = pr
	// Keep Content-Type as text/event-stream
	return resp, nil
}

func translateStreamEvents(reader io.Reader, writer io.Writer, model string) {
	scanner := bufio.NewScanner(reader)
	// Increase buffer for large streaming payloads
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var currentEvent string
	var msgID string
	var msgModel string

	// Track tool call indices: block index → tool_calls array index
	toolCallIndex := 0
	// Track block index → tool name mapping for streaming
	blockToolNames := map[int]string{}
	blockToolIDs := map[int]string{}

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "" {
			continue
		}

		switch currentEvent {
		case "message_start":
			var evt anthropicStreamMessageStart
			if json.Unmarshal([]byte(data), &evt) != nil {
				continue
			}
			msgID = "chatcmpl-" + strings.TrimPrefix(evt.Message.ID, "msg_")
			msgModel = evt.Message.Model
			if msgModel == "" {
				msgModel = model
			}
			// Emit initial chunk with role
			chunk := openaiStreamChunk{
				ID:     msgID,
				Object: "chat.completion.chunk",
				Model:  msgModel,
				Choices: []openaiChoice{
					{Index: 0, Delta: &openaiRespDelta{Role: "assistant"}},
				},
			}
			writeSSEChunk(writer, chunk)

		case "content_block_start":
			var evt anthropicStreamContentBlockStart
			if json.Unmarshal([]byte(data), &evt) != nil {
				continue
			}
			if evt.ContentBlock.Type == "tool_use" {
				blockToolNames[evt.Index] = evt.ContentBlock.Name
				blockToolIDs[evt.Index] = evt.ContentBlock.ID
				idx := toolCallIndex
				chunk := openaiStreamChunk{
					ID:     msgID,
					Object: "chat.completion.chunk",
					Model:  msgModel,
					Choices: []openaiChoice{
						{Index: 0, Delta: &openaiRespDelta{
							ToolCalls: []openaiToolCall{
								{
									Index: &idx,
									ID:    evt.ContentBlock.ID,
									Type:  "function",
									Function: openaiToolCallFn{
										Name:      evt.ContentBlock.Name,
										Arguments: "",
									},
								},
							},
						}},
					},
				}
				writeSSEChunk(writer, chunk)
				toolCallIndex++
			}
			// text blocks: no initial chunk needed

		case "content_block_delta":
			var evt anthropicStreamContentBlockDelta
			if json.Unmarshal([]byte(data), &evt) != nil {
				continue
			}

			switch evt.Delta.Type {
			case "text_delta":
				chunk := openaiStreamChunk{
					ID:     msgID,
					Object: "chat.completion.chunk",
					Model:  msgModel,
					Choices: []openaiChoice{
						{Index: 0, Delta: &openaiRespDelta{Content: evt.Delta.Text}},
					},
				}
				writeSSEChunk(writer, chunk)

			case "input_json_delta":
				// Find the correct tool_calls index for this block
				idx := findToolIndex(evt.Index, blockToolIDs, toolCallIndex)
				chunk := openaiStreamChunk{
					ID:     msgID,
					Object: "chat.completion.chunk",
					Model:  msgModel,
					Choices: []openaiChoice{
						{Index: 0, Delta: &openaiRespDelta{
							ToolCalls: []openaiToolCall{
								{
									Index: &idx,
									Function: openaiToolCallFn{
										Arguments: evt.Delta.PartialJSON,
									},
								},
							},
						}},
					},
				}
				writeSSEChunk(writer, chunk)
			}

		case "content_block_stop":
			// No-op for OpenAI format

		case "message_delta":
			var evt anthropicStreamMessageDelta
			if json.Unmarshal([]byte(data), &evt) != nil {
				continue
			}
			finishReason := mapStopReason(evt.Delta.StopReason)
			chunk := openaiStreamChunk{
				ID:     msgID,
				Object: "chat.completion.chunk",
				Model:  msgModel,
				Choices: []openaiChoice{
					{Index: 0, FinishReason: &finishReason},
				},
			}
			if evt.Usage != nil {
				chunk.Usage = &openaiUsage{
					PromptTokens:     evt.Usage.InputTokens,
					CompletionTokens: evt.Usage.OutputTokens,
					TotalTokens:      evt.Usage.InputTokens + evt.Usage.OutputTokens,
				}
			}
			writeSSEChunk(writer, chunk)

		case "message_stop":
			fmt.Fprint(writer, "data: [DONE]\n\n")
			return

		case "error":
			var evt anthropicStreamError
			if json.Unmarshal([]byte(data), &evt) == nil {
				// Write the error as a final chunk with content
				errChunk := openaiStreamChunk{
					ID:     msgID,
					Object: "chat.completion.chunk",
					Model:  msgModel,
					Choices: []openaiChoice{
						{Index: 0, Delta: &openaiRespDelta{
							Content: "[Anthropic Error: " + evt.Error.Message + "]",
						}},
					},
				}
				writeSSEChunk(writer, errChunk)
			}
			fmt.Fprint(writer, "data: [DONE]\n\n")
			return

		case "ping":
			// Ignore Anthropic ping events
		}

		currentEvent = ""
	}

	// If we exit the scanner without message_stop, send [DONE] anyway
	fmt.Fprint(writer, "data: [DONE]\n\n")
}

// findToolIndex maps Anthropic's block index to the OpenAI tool_calls array index.
func findToolIndex(blockIndex int, blockToolIDs map[int]string, totalTools int) int {
	// Count how many tool blocks appeared before blockIndex
	idx := 0
	for i := 0; i < blockIndex; i++ {
		if _, isToolBlock := blockToolIDs[i]; isToolBlock {
			idx++
		}
	}
	return idx
}

func writeSSEChunk(w io.Writer, chunk openaiStreamChunk) {
	data, err := json.Marshal(chunk)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", data)
}
