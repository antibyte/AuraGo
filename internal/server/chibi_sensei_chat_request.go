package server

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"aurago/internal/chibisensei"
)

const (
	chibiChatMaxTargets   = 2
	chibiChatMaxTurns     = 20    // user messages per conversation
	chibiChatMaxMessages  = 64    // messages per conversation
	chibiChatMaxChars     = 16000 // Unicode code points per conversation, system message included
	chibiChatMaxTokensCap = 1024
)

type chibiChatSamplingIn struct {
	Temperature *float64 `json:"temperature"`
	TopP        *float64 `json:"top_p"`
	TopK        *int     `json:"top_k"`
	MaxTokens   *int     `json:"max_tokens"`
	Seed        *int64   `json:"seed"`
}

type chibiChatTargetIn struct {
	Kind     string                `json:"kind"`
	Messages []chibisensei.Message `json:"messages"`
}

type chibiChatRequest struct {
	RunID    string               `json:"run_id"`
	Targets  []chibiChatTargetIn  `json:"targets"`
	Sampling *chibiChatSamplingIn `json:"sampling"`
}

// chibiChatInvalid is the single 422 issue of the chat route (message key chibi_sensei.chat.invalid).
func chibiChatInvalid(field, format string, args ...any) *chibisensei.ValidationError {
	return &chibisensei.ValidationError{Issues: []chibisensei.Issue{{
		Field: field, MessageKey: "chibi_sensei.chat.invalid", Message: fmt.Sprintf(format, args...)}}}
}

// validateChibiChat checks a decoded chat request; the first failure wins.
func validateChibiChat(req *chibiChatRequest) *chibisensei.ValidationError {
	if !chibisensei.ValidID(req.RunID) {
		return chibiChatInvalid("run_id", "The run id is not valid.")
	}
	if n := len(req.Targets); n < 1 || n > chibiChatMaxTargets {
		return chibiChatInvalid("targets", "Send one or two targets.")
	}
	seen := map[string]bool{}
	for i, t := range req.Targets {
		field := fmt.Sprintf("targets[%d]", i)
		if t.Kind != "tuned" && t.Kind != "base" {
			return chibiChatInvalid(field+".kind", "The kind must be tuned or base.")
		}
		if seen[t.Kind] {
			return chibiChatInvalid(field+".kind", "Each kind may appear once.")
		}
		seen[t.Kind] = true
		if err := validateChibiConversation(field+".messages", t.Messages); err != nil {
			return err
		}
	}
	if sm := req.Sampling; sm != nil {
		switch {
		case sm.Temperature != nil && (*sm.Temperature < 0 || *sm.Temperature > 2):
			return chibiChatInvalid("sampling.temperature", "The temperature must be between 0 and 2.")
		case sm.TopP != nil && (*sm.TopP <= 0 || *sm.TopP > 1):
			return chibiChatInvalid("sampling.top_p", "top_p must be above 0 and at most 1.")
		case sm.TopK != nil && (*sm.TopK < 0 || *sm.TopK > 200):
			return chibiChatInvalid("sampling.top_k", "top_k must be between 0 and 200.")
		case sm.MaxTokens != nil && (*sm.MaxTokens < 1 || *sm.MaxTokens > chibiChatMaxTokensCap):
			return chibiChatInvalid("sampling.max_tokens", "max_tokens must be between 1 and %d.", chibiChatMaxTokensCap)
		case sm.Seed != nil && *sm.Seed < 0:
			return chibiChatInvalid("sampling.seed", "The seed must not be negative.")
		}
	}
	return nil
}

// validateChibiConversation applies the dataset-row role rules: an optional single leading system message, then user
// and assistant alternating, starting and ending with a user message. Length is counted in Unicode code points.
func validateChibiConversation(field string, msgs []chibisensei.Message) *chibisensei.ValidationError {
	if len(msgs) == 0 || len(msgs) > chibiChatMaxMessages {
		return chibiChatInvalid(field, "A conversation needs 1 to %d messages.", chibiChatMaxMessages)
	}
	chars, users := 0, 0
	next := "user"
	for i, m := range msgs {
		at := fmt.Sprintf("%s[%d]", field, i)
		if strings.TrimSpace(m.Content) == "" || strings.ContainsRune(m.Content, 0) {
			return chibiChatInvalid(at+".content", "A message needs text without NUL characters.")
		}
		chars += utf8.RuneCountInString(m.Content)
		switch {
		case i == 0 && m.Role == "system":
			continue // next stays "user"
		case m.Role != next:
			return chibiChatInvalid(at+".role", "The roles must alternate between user and assistant, starting with user.")
		}
		if m.Role == "user" {
			users++
			next = "assistant"
		} else {
			next = "user"
		}
	}
	switch {
	case msgs[len(msgs)-1].Role != "user":
		return chibiChatInvalid(field, "The conversation must end with a user message.")
	case users > chibiChatMaxTurns:
		return chibiChatInvalid(field, "At most %d user messages are allowed.", chibiChatMaxTurns)
	case chars > chibiChatMaxChars:
		return chibiChatInvalid(field, "The conversation is longer than %d characters.", chibiChatMaxChars)
	}
	return nil
}
