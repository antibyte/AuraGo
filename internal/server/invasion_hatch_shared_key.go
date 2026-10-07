package server

import (
	"errors"

	"aurago/internal/invasion"
)

// replaceEggSharedKey stores a hatch's new shared key and returns the step
// that undoes it after a failed deploy. The undo puts the previous key back
// only when the deploy error carries invasion.ErrEggConfigNotDelivered (the
// new egg configuration never left the master, so the egg that ran before
// the hatch still uses the previous key), a previous key was stored, and the
// vault still holds this hatch's key; a rotation or another hatch in the
// meantime wins. It never runs after a rollback. Reports whether it restored.
//
// storeEggSharedKey also drops the rotation candidates (_next, _prev,
// _prev_at). The still-running egg may authenticate with one of them (a
// rotation it persisted but the master never committed, or an older binary
// within the _prev grace), so the undo puts back every candidate slot the
// hatch emptied, unless something wrote that slot in the meantime.
func (s *Server) replaceEggSharedKey(nestID, newKey string) (func(deployErr error) bool, error) {
	previous, hasPrevious := s.storedEggSharedKey(nestID)
	previousSlots, slotsErr := s.readEggKeySlots(nestID)
	if err := s.storeEggSharedKey(nestID, newKey); err != nil {
		return nil, err
	}
	return func(deployErr error) bool {
		if !hasPrevious || previous == newKey || !errors.Is(deployErr, invasion.ErrEggConfigNotDelivered) {
			return false
		}
		if err := s.Vault.CompareAndSwapSecret("egg_shared_"+nestID, newKey, previous); err != nil {
			if s.Logger != nil {
				s.Logger.Warn("[Invasion] Could not restore the previous egg shared key after a failed hatch", "nest_id", nestID, "error", err)
			}
			return false
		}
		if slotsErr == nil {
			s.restoreEggKeyCandidates(nestID, previousSlots)
		}
		if s.Logger != nil {
			s.Logger.Info("[Invasion] Restored the previous egg shared key: the failed hatch never delivered the new egg configuration", "nest_id", nestID)
		}
		return true
	}, nil
}

// restoreEggKeyCandidates writes back the rotation candidates of previous
// whose slots are still empty. The vault has no multi-key conditional write,
// so a slot written after the hatch keeps its newer value.
func (s *Server) restoreEggKeyCandidates(nestID string, previous eggKeySlots) {
	current, err := s.readEggKeySlots(nestID)
	if err != nil {
		if s.Logger != nil {
			s.Logger.Warn("[Invasion] Could not restore the egg key rotation candidates after a failed hatch", "nest_id", nestID, "error", err)
		}
		return
	}
	set := map[string]string{}
	if previous.next != "" && current.next == "" {
		set[eggSharedKeySlotName(nestID, eggSharedKeyNextSuffix)] = previous.next
	}
	// _prev and its date are one entry: restore them together or not at all.
	if previous.prev != "" && current.prev == "" && current.prevAt == "" {
		set[eggSharedKeySlotName(nestID, eggSharedKeyPrevSuffix)] = previous.prev
		if previous.prevAt != "" {
			set[eggSharedKeySlotName(nestID, eggSharedKeyPrevAtSuffix)] = previous.prevAt
		}
	}
	if len(set) == 0 {
		return
	}
	if err := s.Vault.WriteSecrets(set, nil); err != nil && s.Logger != nil {
		s.Logger.Warn("[Invasion] Could not restore the egg key rotation candidates after a failed hatch", "nest_id", nestID, "error", err)
	}
}

// storedEggSharedKey returns the nest's current shared key; ok is false when
// none is stored or the vault cannot be read.
func (s *Server) storedEggSharedKey(nestID string) (string, bool) {
	if s.Vault == nil {
		return "", false
	}
	key, err := s.Vault.ReadSecret("egg_shared_" + nestID)
	if err != nil || key == "" {
		return "", false
	}
	return key, true
}
