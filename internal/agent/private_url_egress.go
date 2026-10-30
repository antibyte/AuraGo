package agent

import (
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"aurago/internal/memory"
	"aurago/internal/security"

	openai "github.com/sashabaranov/go-openai"
)

var outboundURL = regexp.MustCompile("(?i)https?://[^\\s<>\"'`]+")
var privateEmail = regexp.MustCompile(`(?i)[a-z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?\.[a-z]{2,63}`)

func recordPrivateHumanInput(session string, input *memory.HumanInput) {
	if input == nil || !input.Valid() || input.SessionID != session {
		return
	}
	modelIsolatedBodies.addWithSource(session, []string{input.Text}, true, true)
}

// RecordExternalIngress is called only by authenticated server-owned mail relays.
func RecordExternalIngress(session string, source security.ExternalSource, text string) {
	if session == "" || source != security.ExternalEmail {
		return
	}
	modelIsolatedBodies.noteExternalContact(session)
	modelIsolatedBodies.addWithSource(session, copiedSpanViews(text), true, false)
}

func (s *isolatedBodyStore) noteExternalContact(id string) {
	if id == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.contacts == nil {
		s.contacts = map[string]time.Time{}
	}
	s.contacts[id] = s.clock()
	for len(s.contacts) > untrustedContactMax {
		oldest, first := "", true
		var at time.Time
		for sid, t := range s.contacts {
			if sid != id && (first || t.Before(at)) {
				oldest, at, first = sid, t, false
			}
		}
		if first {
			break
		}
		delete(s.contacts, oldest)
	}
}

// hadContact reports whether the session had untrusted contact within the window.
func (s *isolatedBodyStore) hadContact(id string, within time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	at, ok := s.contacts[id]
	return ok && s.clock().Sub(at) < within
}

func (s *isolatedBodyStore) privateURLContext(id string) (private, human []string, contact bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	at, hasContact := s.contacts[id]
	contact = hasContact && s.clock().Sub(at) < untrustedReadTTL
	session := s.sessions[id]
	if session == nil {
		return
	}
	s.touchLocked(id)
	for _, entry := range session.entries {
		if entry.private {
			private = append(private, entry.views...)
		}
		if entry.human {
			human = append(human, entry.views...)
		}
	}
	return
}

func egressWords(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) })
}

func privateWordsInURL(private, target string, count int) bool {
	words := egressWords(target)
	if len(words) < count {
		return false
	}
	private = " " + strings.Join(egressWords(private), " ") + " "
	for i := 0; i+count <= len(words); i++ {
		if strings.Contains(private, " "+strings.Join(words[i:i+count], " ")+" ") {
			return true
		}
	}
	return false
}

func cleanLiteralURL(s string) string { return strings.TrimRight(s, ".,;!)]}") }

// Query separators may be valid email characters, so inspect URL components too.
func privateURLAddresses(text string) []string {
	var addresses []string
	for _, form := range copiedFetchForms(text) {
		addresses = append(addresses, privateEmail.FindAllString(form, -1)...)
		for _, literal := range outboundURL.FindAllString(form, -1) {
			u, err := url.Parse(cleanLiteralURL(literal))
			if err != nil {
				continue
			}
			parts := append(strings.Split(u.Path, "/"), u.Fragment)
			if u.User != nil {
				parts = append(parts, u.User.Username())
			}
			for _, query := range []string{u.RawQuery, u.Fragment} {
				values, _ := url.ParseQuery(query)
				for _, list := range values {
					parts = append(parts, list...)
				}
			}
			for _, part := range parts {
				addresses = append(addresses, privateEmail.FindAllString(part, -1)...)
			}
		}
	}
	return addresses
}

// privateURLExfiltration also covers literal URLs in execution arguments. It does
// not claim to understand URLs computed by arbitrary shell/Python programs.
func privateURLExfiltration(tc ToolCall, dc *DispatchContext) bool {
	if dc == nil || dc.SessionID == "" {
		return false
	}
	private, human, contact := modelIsolatedBodies.privateURLContext(dc.SessionID)
	if !contact || len(private) == 0 {
		return false
	}
	targets := copiedFetchTargets(tc)
	// Executable input can make requests; ordinary local file content cannot.
	if a2aCodeExecution(tc) || oneOf(tc.Operation, "exec", "execute", "run_command", "execute_command", "run_shell_task", "run_desktop_task", "start_job", "job_input", "eval") {
		for _, v := range outboundURL.FindAllString(copiedSpanArguments(tc, false), -1) {
			targets = append(targets, cleanLiteralURL(v))
		}
	}
	if len(targets) == 0 {
		return false
	}
	var addresses []string
	for i, text := range private {
		addresses = append(addresses, privateURLAddresses(text)...)
		private[i] = outboundURL.ReplaceAllString(text, " ")
	}
	allowed := map[string]bool{}
	for _, v := range human {
		for _, u := range outboundURL.FindAllString(v, -1) {
			allowed[cleanLiteralURL(u)] = true
		}
	}
	for _, target := range targets {
		if allowed[target] {
			continue
		}
		if len(target) > 64<<10 {
			return true
		}
		for _, form := range copiedFetchForms(target) {
			for _, address := range addresses {
				if strings.Contains(form, address) {
					return true
				}
			}
			u, err := url.Parse(form)
			if err != nil {
				continue
			}
			for _, text := range private {
				if privateWordsInURL(text, u.RawQuery+" "+u.Fragment, 4) || privateWordsInURL(text, u.Path, 6) {
					return true
				}
			}
		}
	}
	return false
}

// thirdPartyIngressSources are the MessageSource values of runs started by a
// message a third party can author. Heartbeat, cron, planner notifications,
// Uptime Kuma and camera detection are system-generated and not listed.
var thirdPartyIngressSources = map[string]bool{
	"webhook": true, "a2a": true, "email": true, "agentmail": true, "sms": true,
	"mqtt": true, "frigate": true, "space_agent_bridge": true,
}

// missionTriggerData matches the trigger block appendIsolatedTriggerContext adds
// when a mission was started with event data: the header and a fenced payload.
var missionTriggerData = regexp.MustCompile(`\[Trigger Context: [^\]\n]*\]\s*<external_data>`)

// noteIngressProvenance taints the run's session when it was started by a third-
// party ingress, or by a mission whose request carries trigger data. The source
// label decides; a fence in a Loopback message proves nothing (Loopback fences
// every message), so only mission runs are read for the trigger block.
func noteIngressProvenance(runCfg RunConfig, req openai.ChatCompletionRequest) {
	if runCfg.SessionID == "" {
		return
	}
	// A queued run (follow_up, wait_for_event) inherits the taint of the session
	// that queued it; the flag came in over the authenticated internal headers.
	if runCfg.UntrustedProvenance || requestCarriesUntrustedToolResult(req.Messages) {
		modelIsolatedBodies.noteExternalContact(runCfg.SessionID)
	}
	source := strings.ToLower(strings.TrimSpace(runCfg.MessageSource))
	if thirdPartyIngressSources[source] {
		modelIsolatedBodies.noteExternalContact(runCfg.SessionID)
		return
	}
	if !runCfg.IsMission && source != "mission" {
		return
	}
	for _, m := range req.Messages {
		if m.Role == openai.ChatMessageRoleUser && missionTriggerData.MatchString(m.Content) {
			modelIsolatedBodies.noteExternalContact(runCfg.SessionID)
			return
		}
	}
}

// taintedForQueuedRun reports whether work queued from this dispatch context
// continues third-party influence: the session, or for a co-agent its parent
// session, read untrusted content within the taint window.
func taintedForQueuedRun(dc *DispatchContext) bool {
	if dc == nil {
		return false
	}
	return sessionHadUntrustedContact(dc.SessionID, untrustedTaintTTL) ||
		(dc.ParentSessionID != "" && sessionHadUntrustedContact(dc.ParentSessionID, untrustedTaintTTL))
}

// requestCarriesUntrustedToolResult reports whether the messages hold a tool
// result that is still in context and came from third-party content: a tool of
// security.UntrustedContentTool, a shell or code call that fetched from the
// network, or an internal store output the Guardian delivered with a warning.
// A tool that cannot be named does not count. This restores the taint after a
// restart or once the contact timestamp is older than the taint window.
func requestCarriesUntrustedToolResult(messages []openai.ChatCompletionMessage) bool {
	type call struct{ name, args string }
	calls := map[string]call{}
	for _, m := range messages {
		for _, c := range m.ToolCalls {
			if c.ID != "" {
				calls[c.ID] = call{c.Function.Name, c.Function.Arguments}
			}
		}
	}
	for _, m := range messages {
		if m.Role != openai.ChatMessageRoleTool {
			continue
		}
		name, args := m.Name, ""
		if c, ok := calls[m.ToolCallID]; ok {
			if name == "" {
				name = c.name
			}
			args = c.args
		}
		name = strings.ToLower(strings.TrimSpace(name))
		switch {
		case name == "":
		case security.IsInternalToolOutput(name):
			if internalOutputWarned(m.Content) {
				return true
			}
		case security.UntrustedContentTool(name, ""):
			return true
		case networkCapableTools[name] && networkFetchPattern.MatchString(args):
			return true
		}
	}
	return false
}
