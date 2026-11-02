package agent

import "time"

// isolatedProvenance preserves trusted origin flags and original expiry times.
// It holds at most the existing per-session body/read limits.
type isolatedProvenance struct {
	entries []isolatedEntry
	reads   []untrustedRead
	bytes   int
	contact time.Time
}

func (s *isolatedBodyStore) snapshot(id string) isolatedProvenance {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := isolatedProvenance{contact: s.contacts[id]}
	if session := s.sessions[id]; session != nil {
		p.bytes = session.bytes
		p.reads = append([]untrustedRead(nil), session.reads...)
		for _, entry := range session.entries {
			entry.views = append([]string(nil), entry.views...)
			p.entries = append(p.entries, entry)
		}
	}
	return p
}

// inherit installs a snapshot only into a fresh child identity. It uses the same
// global eviction limits and never renews an external contact's TTL.
func (s *isolatedBodyStore) inherit(id string, p isolatedProvenance) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session := s.sessionLocked(id)
	s.total -= session.bytes
	*session = isolatedSession{reads: append([]untrustedRead(nil), p.reads...), bytes: p.bytes}
	for _, entry := range p.entries {
		entry.views = append([]string(nil), entry.views...)
		session.entries = append(session.entries, entry)
	}
	s.total += p.bytes
	if s.contacts == nil {
		s.contacts = make(map[string]time.Time)
	}
	delete(s.contacts, id)
	if !p.contact.IsZero() && s.clock().Sub(p.contact) < untrustedTaintTTL {
		s.contacts[id] = p.contact
	}
	s.evictContactsLocked(id)
	s.evictLocked(id)
}
