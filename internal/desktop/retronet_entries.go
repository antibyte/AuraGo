package desktop

import (
	"context"
	"fmt"
	"sync"

	"aurago/internal/retronet"
)

// retroNetEntriesMu serializes server-side read-modify-write updates of the
// retronet.entries setting across all Service instances sharing the database.
var retroNetEntriesMu sync.Mutex

// RetroNetEntries reads the validated own entries.
func (s *Service) RetroNetEntries(ctx context.Context) ([]retronet.Entry, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	settings, err := s.listSettings(ctx)
	if err != nil {
		return nil, err
	}
	return retronet.ParseEntriesDocument(settings[retronet.EntriesSetting])
}

// SetRetroNetHostKey stores a first-contact fingerprint into the own entry (read-modify-write
// under a package-level mutex, source SourceSystem). Unknown IDs or entries that already have a
// key return an error.
func (s *Service) SetRetroNetHostKey(ctx context.Context, entryID, fingerprint string) error {
	retroNetEntriesMu.Lock()
	defer retroNetEntriesMu.Unlock()
	entries, err := s.RetroNetEntries(ctx)
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
	return s.SetSetting(ctx, retronet.EntriesSetting, document, SourceSystem)
}
