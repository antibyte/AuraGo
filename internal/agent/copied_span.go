package agent

import (
	"aurago/internal/security"
	"context"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	copiedSpanRunes = 48
	// copiedSpanSessionBytes is the normalised text kept per session. The oldest
	// outputs are evicted first; one output larger than the budget keeps its start.
	copiedSpanSessionBytes = 256 << 10
	// copiedSpanTotalBytes bounds all sessions together, copiedSpanMaxSessions their number.
	// The session unused for the longest time goes first (see touchLocked).
	copiedSpanTotalBytes  = 16 << 20
	copiedSpanMaxSessions = 256
	// untrustedReadMax and untrustedReadTTL bound the record of external tools whose
	// output reached the model in a session.
	untrustedReadMax = 8
	untrustedReadTTL = time.Hour
	// untrustedTaintTTL is how long a session counts as tainted after it read
	// third-party content (security.UntrustedContentTool). The URL exfiltration
	// check keeps the shorter untrustedReadTTL.
	untrustedTaintTTL = 24 * time.Hour
	// untrustedContactMax bounds the contact timestamps, kept apart from the
	// copied-span content so its LRU eviction cannot drop the taint.
	untrustedContactMax = 4096
)

// isolatedBodyStore keeps the model-visible isolated outputs of each session, in
// full up to a byte budget, and the external tools that produced them. The next
// high-risk call is checked against the outputs. The current call is recorded
// only after it returns, so it cannot block itself.
type isolatedBodyStore struct {
	mu       sync.Mutex
	order    []string // Least recently used first.
	sessions map[string]*isolatedSession
	total    int
	now      func() time.Time
	contacts map[string]time.Time // Last untrusted contact per session; bounded by untrustedContactMax.
}

type isolatedSession struct {
	entries []isolatedEntry // Oldest first; trusted provenance travels with the text.
	bytes   int
	reads   []untrustedRead // Oldest first, one per tool.
}

type isolatedEntry struct {
	views          []string
	private, human bool
}

type untrustedRead struct {
	tool string
	at   time.Time
}

var modelIsolatedBodies = &isolatedBodyStore{sessions: map[string]*isolatedSession{}, now: time.Now}

// recordIsolatedModelBody indexes one tool output for the copied-span check.
func recordIsolatedModelBody(sessionID, output string) {
	recordIsolatedModelOutput(sessionID, "", "", output)
}

// recordIsolatedModelOutput indexes one tool output and, when tool is set, notes that
// this external tool's content reached the model. A quarantine notice is neither.
func recordIsolatedModelOutput(sessionID, tool, operation, output string) {
	if sessionID == "" {
		return
	}
	payload, ok := isolatedModelPayload(output)
	if !ok {
		return
	}
	// AuraGo's own stores are neither indexed for the copied-span check nor
	// noted as untrusted reads. Memory can still hold text that came from outside
	// (indexed files, recaps of earlier sessions): an internal output the Guardian
	// delivered with a warning taints the session like an untrusted read.
	if security.IsInternalToolOutput(tool) {
		if internalOutputWarned(output) {
			modelIsolatedBodies.noteExternalContact(sessionID)
		}
		return
	}
	if name := untrustedReadName(tool); name != "" {
		modelIsolatedBodies.noteRead(sessionID, name)
	}
	source := security.ExternalToolSource(tool, operation)
	if security.UntrustedContentTool(tool, operation) {
		modelIsolatedBodies.noteExternalContact(sessionID)
	}
	var views []string
	for _, view := range copiedSpanViews(payload) {
		if view != "" {
			views = append(views, view)
		}
	}
	if len(views) > 0 {
		modelIsolatedBodies.addWithSource(sessionID, views, source != security.ExternalWeb, false)
	}
}

// isolatedModelPayload recovers the content the model saw from an isolated tool
// output. The handler or SanitizeToolOutput wrapped it in <external_data>. Only the
// notice itself, which starts the output, is skipped: a page that merely prints
// the notice's words must not stop its own text from being recorded.
func isolatedModelPayload(output string) (string, bool) {
	if !strings.Contains(output, "<external_data>") {
		return "", false
	}
	payload, isolated := toolResultPayload(output)
	if isolated {
		return payload, true
	}
	if strings.HasPrefix(payload, "[QUARANTINE NOTICE]") {
		return "", false
	}
	return output, true
}

// untrustedReadName keeps a tool name that is safe to show in an approval and to
// hand to the guardian.
func untrustedReadName(tool string) string {
	tool = strings.ToLower(strings.TrimSpace(tool))
	if tool == "" || len(tool) > 48 {
		return ""
	}
	for _, r := range tool {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || strings.ContainsRune("_.:/-", r)) {
			return ""
		}
	}
	return tool
}

func (s *isolatedBodyStore) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

func (s *isolatedBodyStore) sessionLocked(sessionID string) *isolatedSession {
	session, ok := s.sessions[sessionID]
	if !ok {
		session = &isolatedSession{}
		s.sessions[sessionID] = session
		s.order = append(s.order, sessionID)
	} else {
		s.touchLocked(sessionID)
	}
	return session
}

// touchLocked marks a session as just used by moving it to the back of the order.
// Recording an output, noting a read and checking a call all count as use, so a
// session that is still working is never the one evicted for a crowd of idle ones.
func (s *isolatedBodyStore) touchLocked(sessionID string) {
	for i, id := range s.order {
		if id == sessionID {
			copy(s.order[i:], s.order[i+1:])
			s.order[len(s.order)-1] = sessionID
			return
		}
	}
}

// evictLocked drops the least recently used sessions, never keep, until the count
// and the byte total fit.
func (s *isolatedBodyStore) evictLocked(keep string) {
	for (len(s.sessions) > copiedSpanMaxSessions || s.total > copiedSpanTotalBytes) && len(s.order) > 1 {
		at := 0
		if s.order[at] == keep {
			at = 1
		}
		drop := s.order[at]
		s.order = append(s.order[:at], s.order[at+1:]...)
		if session := s.sessions[drop]; session != nil {
			s.total -= session.bytes
		}
		delete(s.sessions, drop)
	}
}

func truncateCopiedView(view string, limit int) string {
	if len(view) <= limit {
		return view
	}
	for limit > 0 && !utf8.RuneStart(view[limit]) {
		limit--
	}
	return view[:limit]
}

func (s *isolatedBodyStore) add(sessionID string, views []string) {
	s.addWithSource(sessionID, views, false, false)
}

func (s *isolatedBodyStore) addWithSource(sessionID string, views []string, private, human bool) {
	if len(views) == 0 || sessionID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	session := s.sessionLocked(sessionID)
	entry := make([]string, len(views))
	size := 0
	for i, view := range views {
		entry[i] = truncateCopiedView(view, copiedSpanSessionBytes/len(views))
		size += len(entry[i])
	}
	session.entries = append(session.entries, isolatedEntry{entry, private, human})
	session.bytes += size
	s.total += size
	for session.bytes > copiedSpanSessionBytes && len(session.entries) > 1 {
		dropped := 0
		for _, view := range session.entries[0].views {
			dropped += len(view)
		}
		session.entries = session.entries[1:]
		session.bytes -= dropped
		s.total -= dropped
	}
	s.evictLocked(sessionID)
}

func (s *isolatedBodyStore) noteRead(sessionID, tool string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session := s.sessionLocked(sessionID)
	now := s.clock()
	kept := session.reads[:0]
	for _, read := range session.reads {
		if read.tool != tool && now.Sub(read.at) < untrustedReadTTL {
			kept = append(kept, read)
		}
	}
	session.reads = append(kept, untrustedRead{tool: tool, at: now})
	if len(session.reads) > untrustedReadMax {
		session.reads = session.reads[len(session.reads)-untrustedReadMax:]
	}
	s.evictLocked(sessionID)
}

// views returns every indexed text of the session.
func (s *isolatedBodyStore) views(sessionID string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	session := s.sessions[sessionID]
	if session == nil {
		return nil
	}
	s.touchLocked(sessionID)
	var out []string
	for _, entry := range session.entries {
		if !entry.human {
			out = append(out, entry.views...)
		}
	}
	return out
}

// recentReads returns the external tools read lately, newest first.
func (s *isolatedBodyStore) recentReads(sessionID string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	session := s.sessions[sessionID]
	if session == nil {
		return nil
	}
	s.touchLocked(sessionID)
	now := s.clock()
	var out []string
	for i := len(session.reads) - 1; i >= 0; i-- {
		if now.Sub(session.reads[i].at) < untrustedReadTTL {
			out = append(out, session.reads[i].tool)
		}
	}
	return out
}

func clearIsolatedModelBodies(sessionID string) {
	modelIsolatedBodies.mu.Lock()
	defer modelIsolatedBodies.mu.Unlock()
	if session := modelIsolatedBodies.sessions[sessionID]; session != nil {
		modelIsolatedBodies.total -= session.bytes
	}
	delete(modelIsolatedBodies.sessions, sessionID)
	delete(modelIsolatedBodies.contacts, sessionID)
	kept := modelIsolatedBodies.order[:0]
	for _, id := range modelIsolatedBodies.order {
		if id != sessionID {
			kept = append(kept, id)
		}
	}
	modelIsolatedBodies.order = kept
}

// untrustedReadsNote names the external tools whose content reached the model in
// this session, for the approval dialog and the LLM Guardian context. It is
// provenance, not a gate. It is empty when nothing external was read lately.
func untrustedReadsNote(sessionID string) string {
	if sessionID == "" {
		return ""
	}
	tools := modelIsolatedBodies.recentReads(sessionID)
	if len(tools) == 0 {
		return ""
	}
	return "untrusted content read earlier in this session: " + strings.Join(tools, ", ")
}

// guardianContextShownBytes is what the LLM Guardian prompt keeps of a check's
// context; the note must fit in it to reach the judge whole.
const guardianContextShownBytes = 200

// guardianContextWithReads appends the untrusted-read note to the context of an
// LLM Guardian check, shortening the user message to make room for it.
func guardianContextWithReads(userContext, sessionID string) string {
	note := untrustedReadsNote(sessionID)
	if note == "" {
		return userContext
	}
	if userContext == "" {
		return note
	}
	return truncateUTF8ToLimit(userContext, max(guardianContextShownBytes-len(note)-1, 60), "") + "\n" + note
}

// copiesViews reports whether text shares a copiedSpanRunes window with any view.
func copiesViews(views []string, text []rune) bool {
	for _, view := range views {
		if sharesWindow([]rune(view), text) {
			return true
		}
	}
	return false
}

func copiedIsolatedSpan(tc ToolCall, dc *DispatchContext) bool {
	if dc == nil || dc.SessionID == "" {
		return false
	}
	views := modelIsolatedBodies.views(dc.SessionID)
	if len(views) == 0 {
		return false
	}
	args := []rune(copiedSpanArgumentText(tc))
	if len(args) < copiedSpanRunes {
		return false
	}
	return copiesViews(views, args)
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

// sessionHadUntrustedContact reports whether the session read third-party
// content (security.UntrustedContentTool) or received an untrusted ingress
// within the given window.
func sessionHadUntrustedContact(sessionID string, within time.Duration) bool {
	if sessionID == "" {
		return false
	}
	return modelIsolatedBodies.hadContact(sessionID, within)
}

// internalOutputWarned reports whether SanitizeToolOutput appended its warning
// line behind the fence of an internal tool output. Text inside the fence that
// imitates the line can only taint the session, never clear it.
func internalOutputWarned(output string) bool {
	end := strings.LastIndex(output, "</external_data>")
	return end >= 0 && strings.Contains(output[end:], "\n[SECURITY WARNING:")
}
