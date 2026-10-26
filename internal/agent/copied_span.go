package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	copiedSpanRunes       = 48
	copiedSpanBodies      = 3
	copiedSpanMaxBytes    = 4000
	copiedSpanMaxRunes    = 1500
	copiedSpanMaxSessions = 256
)

// isolatedBodyStore keeps the last few model-visible isolated bodies per session.
// The next high-risk call is checked against them. The current call is recorded
// only after it returns, so it cannot block itself.
type isolatedBodyStore struct {
	mu       sync.Mutex
	order    []string
	sessions map[string][]string
}

var modelIsolatedBodies = &isolatedBodyStore{sessions: map[string][]string{}}

func recordIsolatedModelBody(sessionID, output string) {
	if sessionID == "" || !strings.Contains(output, "<external_data>") || strings.Contains(output, "[QUARANTINE NOTICE]") {
		return
	}
	body := boundCopiedSpanBody(output)
	if utf8.RuneCountInString(body) < copiedSpanRunes {
		return
	}
	modelIsolatedBodies.add(sessionID, body)
}

func boundCopiedSpanBody(output string) string {
	runes := []rune(output)
	if len(runes) > copiedSpanMaxRunes {
		runes = runes[:copiedSpanMaxRunes]
	}
	body := string(runes)
	for len(body) > copiedSpanMaxBytes && len(runes) > 0 {
		runes = runes[:len(runes)-1]
		body = string(runes)
	}
	return body
}

func (s *isolatedBodyStore) add(sessionID, body string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[sessionID]; !ok {
		s.order = append(s.order, sessionID)
	}
	bodies := append(s.sessions[sessionID], body)
	if len(bodies) > copiedSpanBodies {
		bodies = bodies[len(bodies)-copiedSpanBodies:]
	}
	s.sessions[sessionID] = bodies
	for len(s.sessions) > copiedSpanMaxSessions && len(s.order) > 0 {
		drop := s.order[0]
		s.order = s.order[1:]
		delete(s.sessions, drop)
	}
}

func (s *isolatedBodyStore) bodies(sessionID string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	src := s.sessions[sessionID]
	out := make([]string, len(src))
	copy(out, src)
	return out
}

func clearIsolatedModelBodies(sessionID string) {
	modelIsolatedBodies.mu.Lock()
	defer modelIsolatedBodies.mu.Unlock()
	delete(modelIsolatedBodies.sessions, sessionID)
	kept := modelIsolatedBodies.order[:0]
	for _, id := range modelIsolatedBodies.order {
		if id != sessionID {
			kept = append(kept, id)
		}
	}
	modelIsolatedBodies.order = kept
}

func copiedIsolatedSpan(tc ToolCall, dc *DispatchContext) bool {
	if dc == nil || dc.SessionID == "" {
		return false
	}
	args := toolCallArgumentJSON(tc)
	if utf8.RuneCountInString(args) < copiedSpanRunes {
		return false
	}
	for _, body := range modelIsolatedBodies.bodies(dc.SessionID) {
		if spanWindowMatches(body, args) {
			return true
		}
	}
	return false
}

func toolCallArgumentJSON(tc ToolCall) string {
	// The session todo and guardian notes ride along on the call. They are not
	// the arguments that will be executed or sent.
	tc.Todo = ""
	tc.GuardianJustification = ""
	tc.GuardianBlocked = false
	tc.GuardianBlockReason = ""
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(tc); err != nil {
		return ""
	}
	return buf.String()
}

func spanWindowMatches(body, args string) bool {
	runes := []rune(body)
	if len(runes) < copiedSpanRunes {
		return false
	}
	for i := 0; i+copiedSpanRunes <= len(runes); i++ {
		window := string(runes[i : i+copiedSpanRunes])
		if strings.TrimSpace(window) == "" {
			continue
		}
		if strings.Contains(args, window) {
			return true
		}
	}
	return false
}

// gateCopiedIsolatedSpan reports whether a high-risk call copies a stored isolated span.
// stop is false when the call may continue. A nil refusal with stop means a still-valid
// one-shot grant covers this exact call.
func gateCopiedIsolatedSpan(ctx context.Context, tc ToolCall, dc *DispatchContext, a ActionDescriptor) (stop bool, refusal *ToolDispatchResult) {
	if !a.HighRisk || !copiedIsolatedSpan(tc, dc) {
		return false, nil
	}
	if grant, ok := ctx.Value(approvedActionKey{}).(approvedActionGrant); ok && grant.SessionID == dc.SessionID && grant.ArgumentsHash == a.ArgumentsHash {
		if grant.expired() {
			return true, actionPolicyRefusal("approval_expired", "The approval expired before execution began. No action was executed.", ToolResultDenied)
		}
		return true, nil
	}
	if preview, _ := ctx.Value(actionPreviewKey{}).(bool); preview {
		return true, actionPolicyRefusal("action_rule_required", "This high-risk action copies isolated tool text. An allow rule does not exempt it. Approve the exact call once it is proposed in an interactive session.", ToolResultDenied)
	}
	if a.RunType != "interactive" {
		return true, actionPolicyRefusal("action_rule_required", "This high-risk unattended action copies isolated tool text. It was blocked without waiting. An allow rule does not exempt that copy.", ToolResultDenied)
	}
	return true, requestCopiedSpanApproval(tc, dc, a)
}
