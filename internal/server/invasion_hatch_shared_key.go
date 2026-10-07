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
func (s *Server) replaceEggSharedKey(nestID, newKey string) (func(deployErr error) bool, error) {
	previous, hasPrevious := s.storedEggSharedKey(nestID)
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
		if s.Logger != nil {
			s.Logger.Info("[Invasion] Restored the previous egg shared key: the failed hatch never delivered the new egg configuration", "nest_id", nestID)
		}
		return true
	}, nil
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
