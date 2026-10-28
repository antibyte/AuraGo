package agent

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"aurago/internal/memory"
	"aurago/internal/security"
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
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessionLocked(id).externalContact = s.clock()
	s.evictLocked(id)
}

func (s *isolatedBodyStore) privateURLContext(id string) (private, human []string, contact bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session := s.sessions[id]
	if session == nil {
		return
	}
	s.touchLocked(id)
	contact = !session.externalContact.IsZero() && s.clock().Sub(session.externalContact) < untrustedReadTTL
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
