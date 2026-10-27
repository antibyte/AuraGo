package agentmail

import (
	"net/mail"
	"strings"
)

// senderVerdict says what the relay does with a message, judged by its sender.
type senderVerdict int

const (
	// senderAllowed: the sender is on the allowlist; the message goes on to the
	// content scans and, if they pass, to the agent.
	senderAllowed senderVerdict = iota
	// senderIgnored: an allowlist is configured and the sender is not on it. The
	// message is dropped without a notice, so a stranger cannot cause agent turns.
	senderIgnored
	// senderUnconfigured: no allowlist exists. Nothing reaches the agent; the
	// owner gets a quarantine notice for every message, so the silent relay is
	// discoverable instead of mysterious.
	senderUnconfigured
)

// normalizeAllowedSenders trims, lower-cases and de-duplicates the entries and
// drops empty ones. Entries that are neither an address nor "@domain" are kept
// as written (and never match), so a typo shows up in the config, not as a
// silently wider list.
func normalizeAllowedSenders(entries []string) []string {
	if len(entries) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(entries))
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry == "" {
			continue
		}
		if _, dup := seen[entry]; dup {
			continue
		}
		seen[entry] = struct{}{}
		out = append(out, entry)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// senderVerdict applies agentmail.allowed_senders to one message.
func (s *Service) senderVerdict(msg Message) senderVerdict {
	if len(s.cfg.AllowedSenders) == 0 {
		return senderUnconfigured
	}
	if senderMatches(s.cfg.AllowedSenders, msg.From.Email) {
		return senderAllowed
	}
	return senderIgnored
}

// senderMatches reports whether the From address is one of the entries: a full
// address (exact) or "@domain" (exactly that domain, never a subdomain or a
// longer name that merely ends in it). A From value that is not a single valid
// address matches nothing. The display name is ignored, so
// `"boss@example.com" <evil@example.net>` is evil@example.net.
func senderMatches(entries []string, from string) bool {
	from = strings.TrimSpace(from)
	if from == "" {
		return false
	}
	parsed, err := mail.ParseAddress(from)
	if err != nil {
		return false
	}
	address := strings.ToLower(parsed.Address)
	at := strings.LastIndex(address, "@")
	if at <= 0 || at == len(address)-1 {
		return false
	}
	domain := address[at:]
	for _, entry := range entries {
		switch {
		case strings.HasPrefix(entry, "@"):
			if len(entry) > 1 && entry == domain {
				return true
			}
		case entry == address:
			return true
		}
	}
	return false
}
