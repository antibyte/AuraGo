package server

import (
	"fmt"
	"strings"
	"time"
)

// Egg shared-key store. Key rotation keeps up to three keys per nest: the
// current key, a staged _next written before the egg is asked to rotate, and
// _prev, the key replaced by a rotation an egg predating the persisted ack
// flag confirmed, dated by _prev_at (RFC 3339, UTC). The egg_shared_ prefix is
// reserved (tools.IsPythonAccessibleSecret, vaultprompt.NormalizeVaultKey) so
// only the master writes these entries.
const (
	eggSharedKeyNextSuffix   = "_next"
	eggSharedKeyPrevSuffix   = "_prev"
	eggSharedKeyPrevAtSuffix = "_prev_at"

	// eggPrevKeyGrace bounds how long the replaced key may still authenticate,
	// once, so an egg on the previous binary (which rotated only in memory)
	// can reconnect after a restart.
	eggPrevKeyGrace = time.Hour
	// eggPrevKeyClockSkew tolerates a small backwards clock step after the
	// rotation that dated _prev.
	eggPrevKeyClockSkew = time.Minute
)

// eggSharedKeyName is the vault entry holding a nest's current shared key.
func eggSharedKeyName(nestID string) string {
	return "egg_shared_" + nestID
}

func eggSharedKeySlotName(nestID, suffix string) string {
	return eggSharedKeyName(nestID) + suffix
}

// eggSharedKeyCandidateNames lists every rotation entry besides the current key.
func eggSharedKeyCandidateNames(nestID string) []string {
	return []string{
		eggSharedKeySlotName(nestID, eggSharedKeyNextSuffix),
		eggSharedKeySlotName(nestID, eggSharedKeyPrevSuffix),
		eggSharedKeySlotName(nestID, eggSharedKeyPrevAtSuffix),
	}
}

func eggPreviousKeyNames(nestID string) []string {
	return []string{eggSharedKeySlotName(nestID, eggSharedKeyPrevSuffix), eggSharedKeySlotName(nestID, eggSharedKeyPrevAtSuffix)}
}

// eggPrevKeyFresh fails closed: an undated, unparseable or future-dated
// (beyond eggPrevKeyClockSkew) _prev is treated as expired.
func eggPrevKeyFresh(rotatedAt string, now time.Time) bool {
	at, err := time.Parse(time.RFC3339, strings.TrimSpace(rotatedAt))
	if err != nil {
		return false
	}
	age := now.Sub(at)
	return age >= -eggPrevKeyClockSkew && age <= eggPrevKeyGrace
}

// eggKeySlots is one consistent read of a nest's key entries; "" means absent.
type eggKeySlots struct {
	current, next, prev, prevAt string
}

// readEggKeySlots reads every slot from a single vault snapshot, so a missing
// entry is told apart from a failed read and nothing is deleted on a read error.
func (s *Server) readEggKeySlots(nestID string) (eggKeySlots, error) {
	if s.Vault == nil {
		return eggKeySlots{}, fmt.Errorf("vault is unavailable")
	}
	snapshot, err := s.Vault.BackupSnapshot()
	if err != nil {
		return eggKeySlots{}, err
	}
	return eggKeySlots{
		current: snapshot[eggSharedKeyName(nestID)],
		next:    snapshot[eggSharedKeySlotName(nestID, eggSharedKeyNextSuffix)],
		prev:    snapshot[eggSharedKeySlotName(nestID, eggSharedKeyPrevSuffix)],
		prevAt:  snapshot[eggSharedKeySlotName(nestID, eggSharedKeyPrevAtSuffix)],
	}, nil
}

func (s *Server) storeEggSharedKey(nestID, sharedKey string) error {
	if s.Vault == nil {
		return fmt.Errorf("failed to store egg shared key: vault is unavailable")
	}
	// Re-hatching is the operator's revocation path: the fresh key replaces
	// every rotation candidate in one atomic write.
	if err := s.Vault.WriteSecrets(map[string]string{eggSharedKeyName(nestID): sharedKey}, eggSharedKeyCandidateNames(nestID)); err != nil {
		return fmt.Errorf("failed to store egg shared key in vault: %w", err)
	}
	return nil
}

// commitEggKeyRotation makes newKey current and drops the staged _next in one
// atomic write. When the egg confirmed persistence every older key is retired.
// Otherwise (an egg predating the flag may hold newKey only in memory and an
// older key on disk) replacedKey is kept as dated _prev — unless a still-fresh
// _prev exists: that older key is what such an egg's disk holds, so it is not
// overwritten by an intermediate key and keeps its original date.
func (s *Server) commitEggKeyRotation(nestID, newKey, replacedKey string, persisted bool) error {
	set := map[string]string{eggSharedKeyName(nestID): newKey}
	remove := []string{eggSharedKeySlotName(nestID, eggSharedKeyNextSuffix)}
	if persisted || replacedKey == "" || replacedKey == newKey {
		return s.Vault.WriteSecrets(set, append(remove, eggPreviousKeyNames(nestID)...))
	}
	now := time.Now()
	slots, err := s.readEggKeySlots(nestID)
	if err != nil {
		return err
	}
	if slots.prev == "" || slots.prev == newKey || !eggPrevKeyFresh(slots.prevAt, now) {
		set[eggSharedKeySlotName(nestID, eggSharedKeyPrevSuffix)] = replacedKey
		set[eggSharedKeySlotName(nestID, eggSharedKeyPrevAtSuffix)] = now.UTC().Format(time.RFC3339)
	}
	return s.Vault.WriteSecrets(set, remove)
}

type eggKeyCandidate struct {
	suffix string // "", eggSharedKeyNextSuffix or eggSharedKeyPrevSuffix
	key    string
}

// eggHandshakeKeyCandidates returns the stored keys an egg may present, in the
// order current, _next, _prev, skipping empty and duplicate values. _prev is
// offered only within eggPrevKeyGrace of the rotation that replaced it; an
// expired _prev is deleted here, whatever the handshake's outcome. A failed
// vault read returns an error and changes nothing.
func (s *Server) eggHandshakeKeyCandidates(nestID string, now time.Time) ([]eggKeyCandidate, error) {
	slots, err := s.readEggKeySlots(nestID)
	if err != nil {
		return nil, err
	}
	if slots.prev != "" && !eggPrevKeyFresh(slots.prevAt, now) {
		slots.prev = ""
		if err := s.Vault.WriteSecrets(nil, eggPreviousKeyNames(nestID)); err != nil {
			s.Logger.Warn("Failed to delete expired previous egg key", "nest_id", nestID, "error", err)
		} else {
			s.Logger.Info("Expired previous egg key deleted", "nest_id", nestID)
		}
	}
	candidates := make([]eggKeyCandidate, 0, 3)
	for _, candidate := range []eggKeyCandidate{
		{suffix: "", key: slots.current},
		{suffix: eggSharedKeyNextSuffix, key: slots.next},
		{suffix: eggSharedKeyPrevSuffix, key: slots.prev},
	} {
		if candidate.key == "" {
			continue
		}
		duplicate := false
		for _, seen := range candidates {
			duplicate = duplicate || seen.key == candidate.key
		}
		if !duplicate {
			candidates = append(candidates, candidate)
		}
	}
	return candidates, nil
}

// reconcileEggSharedKeys records what a successful handshake proved. A staged
// or previous key the egg authenticated with becomes current and the other
// candidates are dropped, so _prev is single-use. A handshake under the
// current key retires _prev as well; _next stays, because a rotation may have
// staged it and be waiting for this egg's ack. The vault has no multi-key
// conditional write (CompareAndSwapSecret covers single keys only), so these
// writes are unconditional.
func (s *Server) reconcileEggSharedKeys(nestID string, matched eggKeyCandidate, candidates []eggKeyCandidate) {
	if matched.suffix == "" {
		for _, candidate := range candidates {
			if candidate.suffix == eggSharedKeyPrevSuffix {
				if err := s.Vault.WriteSecrets(nil, eggPreviousKeyNames(nestID)); err != nil {
					s.Logger.Warn("Failed to retire previous egg key", "nest_id", nestID, "error", err)
				}
				break
			}
		}
		return
	}
	err := s.Vault.WriteSecrets(map[string]string{eggSharedKeyName(nestID): matched.key}, eggSharedKeyCandidateNames(nestID))
	if err != nil {
		s.Logger.Error("Failed to promote egg key candidate; the connection uses it until the next rotation heals the vault", "nest_id", nestID, "candidate", matched.suffix, "error", err)
		return
	}
	if matched.suffix == eggSharedKeyPrevSuffix {
		s.Logger.Warn("Egg authenticated with the pre-rotation key; the rotation was reverted (is the egg running an older binary?)", "nest_id", nestID)
		return
	}
	s.Logger.Info("Promoted staged egg key after handshake", "nest_id", nestID)
}
