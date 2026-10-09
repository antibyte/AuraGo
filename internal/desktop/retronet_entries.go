package desktop

import (
	"context"
	"fmt"
	"sync"

	"aurago/internal/retronet"
)

// retroNetEntriesMu serializes server-side host-key updates; compareAndSetSetting
// keeps them from overwriting concurrent user saves of the retronet.entries setting.
var retroNetEntriesMu sync.Mutex

// retroNetHostKeyAttempts bounds the read-modify-compare-and-set retries of
// SetRetroNetHostKey after a concurrent change of the entries.
const retroNetHostKeyAttempts = 3

// retroNetHostKeyBeforeWrite is a test seam called between the read and the
// write of SetRetroNetHostKey; nil in production.
var retroNetHostKeyBeforeWrite func()

// RetroNetEntries reads the validated own entries.
func (s *Service) RetroNetEntries(ctx context.Context) ([]retronet.Entry, error) {
	_, entries, err := s.retroNetEntriesDocument(ctx)
	return entries, err
}

// retroNetEntriesDocument returns the stored entries document and its parsed entries.
func (s *Service) retroNetEntriesDocument(ctx context.Context) (string, []retronet.Entry, error) {
	if err := s.ensureReady(ctx); err != nil {
		return "", nil, err
	}
	settings, err := s.listSettings(ctx)
	if err != nil {
		return "", nil, err
	}
	document := settings[retronet.EntriesSetting]
	entries, err := retronet.ParseEntriesDocument(document)
	if err != nil {
		return "", nil, err
	}
	return document, entries, nil
}

// SetRetroNetHostKey stores a first-contact fingerprint into the own entry (source SourceSystem).
// It reads, modifies and compare-and-sets the entries document, so a concurrent user save is
// never overwritten; after a lost race it starts again from the new document, at most
// retroNetHostKeyAttempts times. Unknown IDs, non-SSH entries and entries that already have a
// key return an error.
func (s *Service) SetRetroNetHostKey(ctx context.Context, entryID, fingerprint string) error {
	retroNetEntriesMu.Lock()
	defer retroNetEntriesMu.Unlock()
	for attempt := 1; ; attempt++ {
		old, entries, err := s.retroNetEntriesDocument(ctx)
		if err != nil {
			return err
		}
		index := -1
		for i := range entries {
			if entries[i].ID == entryID {
				index = i
				break
			}
		}
		switch {
		case index < 0:
			return fmt.Errorf("unknown retro-net entry %q", entryID)
		case entries[index].Protocol != retronet.ProtocolSSH:
			return fmt.Errorf("retro-net entry %q is not an SSH entry", entryID)
		case entries[index].HostKey != "":
			return fmt.Errorf("retro-net entry %q already has a host key", entryID)
		}
		entries[index].HostKey = fingerprint
		document, err := retronet.EncodeEntriesDocument(entries)
		if err != nil {
			return err
		}
		if retroNetHostKeyBeforeWrite != nil {
			retroNetHostKeyBeforeWrite()
		}
		stored, err := s.compareAndSetSetting(ctx, retronet.EntriesSetting, old, document, SourceSystem)
		if err != nil {
			return err
		}
		if stored {
			return nil
		}
		if attempt >= retroNetHostKeyAttempts {
			return fmt.Errorf("retro-net entries kept changing; host key for %q not stored", entryID)
		}
	}
}
