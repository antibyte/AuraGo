package chatrt

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"aurago/internal/security"
)

const (
	maxStreamLine  = 1 << 20
	maxStreamBytes = 8 << 20
	maxBadLines    = 16
)

// streamChunk is the part of a chat-completion chunk that the runtime reads. Reasoning fields are deliberately
// not decoded: only answer text leaves the package.
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content *string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Timings *struct {
		PromptN            int     `json:"prompt_n"`
		PredictedN         int     `json:"predicted_n"`
		PredictedPerSecond float64 `json:"predicted_per_second"`
	} `json:"timings"`
	Error *upstreamError `json:"error"`
}

// upstreamError is llama-server's error object.
type upstreamError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Type    string `json:"type"`
}

type streamResult struct {
	FinishReason     string
	PromptTokens     int
	CompletionTokens int
	Deltas           int
	PerSecond        float64
	Done             bool
}

var contextPattern = regexp.MustCompile(`(?i)(exceed\w*|too long|larger than|longer than)[^.]{0,40}context|context[^.]{0,40}(exceed\w*|too (long|small)|size)`)

// classifyUpstream maps an error object of llama-server to an error: ErrContext when the type or the message says
// the prompt does not fit the context, otherwise errUpstream with a short scrubbed text. The text is untrusted.
func classifyUpstream(status int, e upstreamError) error {
	if strings.Contains(strings.ToLower(e.Type), "context") || contextPattern.MatchString(e.Message) {
		return ErrContext
	}
	code := status
	if code == 0 {
		code = e.Code
	}
	return fmt.Errorf("%w: status %d, type %q, %s", errUpstream, code, scrubUpstreamText(e.Type), scrubUpstreamText(e.Message))
}

// scrubUpstreamText is the one path of untrusted upstream text into an error: scrubbed first, then cut to 200
// characters (a secret cannot straddle the cut). decodeErrorAnswer and the stream parser both reach it through
// classifyUpstream.
func scrubUpstreamText(s string) string {
	return truncateText(security.Scrub(s), 200)
}

func truncateText(s string, limit int) string {
	r := []rune(strings.ToValidUTF8(s, ""))
	if len(r) > limit {
		return string(r[:limit]) + "..."
	}
	return string(r)
}

// parseStream reads a chat-completion stream tolerantly (see the line rules in the plan) and forwards the answer
// text. It never trusts the framing details: unknown lines are skipped, a few undecodable payloads are tolerated.
func parseStream(r io.Reader, onDelta func(string)) (streamResult, error) {
	var res streamResult
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64<<10), maxStreamLine)
	total, bad := 0, 0
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		total += len(line) + 1
		if total > maxStreamBytes {
			return res, fmt.Errorf("llama-server stream above %d bytes", maxStreamBytes)
		}
		if line == "" || line[0] == ':' || strings.HasPrefix(line, "event:") || strings.HasPrefix(line, "id:") || strings.HasPrefix(line, "retry:") {
			continue
		}
		var payload string
		switch {
		case strings.HasPrefix(line, "data:"):
			payload = strings.TrimSpace(line[len("data:"):])
		case line[0] == '{':
			payload = line
		default:
			continue
		}
		if payload == "[DONE]" {
			res.Done = true
			break
		}
		if payload == "" {
			continue
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			bad++
			if bad > maxBadLines {
				return res, fmt.Errorf("llama-server stream: more than %d undecodable lines", maxBadLines)
			}
			continue
		}
		if chunk.Error != nil {
			return res, classifyUpstream(0, *chunk.Error)
		}
		if len(chunk.Choices) > 0 {
			c := chunk.Choices[0]
			if c.Delta.Content != nil && *c.Delta.Content != "" {
				res.Deltas++
				if onDelta != nil {
					onDelta(*c.Delta.Content)
				}
			}
			if c.FinishReason != nil && *c.FinishReason != "" {
				res.FinishReason = *c.FinishReason
			}
		}
		if chunk.Usage != nil {
			res.PromptTokens = chunk.Usage.PromptTokens
			res.CompletionTokens = chunk.Usage.CompletionTokens
		}
		if chunk.Timings != nil {
			if res.PromptTokens == 0 {
				res.PromptTokens = chunk.Timings.PromptN
			}
			if res.CompletionTokens == 0 {
				res.CompletionTokens = chunk.Timings.PredictedN
			}
			if chunk.Timings.PredictedPerSecond > 0 {
				res.PerSecond = chunk.Timings.PredictedPerSecond
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return res, fmt.Errorf("read llama-server stream: %w", err)
	}
	if !res.Done && res.FinishReason == "" {
		return res, errStreamCut
	}
	return res, nil
}
