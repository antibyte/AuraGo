package prompts

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aurago/internal/personabooster"
	promptsembed "aurago/prompts"
)

const personaPhaseDir = "persona_phases"

type personaPhaseCacheEntry struct {
	catalog  *personabooster.Catalog
	revision string
	ok       bool
	diskSeen bool
	diskSize int64
	diskMod  time.Time
}

var (
	personaPhaseCacheMu sync.Mutex
	personaPhaseCache   = map[string]personaPhaseCacheEntry{}
)

// LoadPersonaPhaseCatalog returns the validated phase catalog for a persona.
// A valid file in <promptsDir>/persona_phases wins; an invalid one logs a
// warning and falls back to the embedded file. Personas without any catalog
// use _generic.yaml; neutral and empty personas have none. The revision
// names the source and digest of the bytes used, for prompt fingerprints.
// Callers must not modify the returned catalog.
func LoadPersonaPhaseCatalog(promptsDir, personaID string, logger *slog.Logger) (*personabooster.Catalog, string, bool) {
	logger = normalizePromptLogger(logger)
	persona, valid := ResolvePersonalityID(personaID)
	if !valid || persona == "" || persona == "neutral" {
		return nil, "", false
	}
	if cat, revision, ok := cachedPersonaPhaseFile(promptsDir, persona, logger); ok {
		return cat, revision, true
	}
	return cachedPersonaPhaseFile(promptsDir, personabooster.GenericPersona, logger)
}

func cachedPersonaPhaseFile(promptsDir, name string, logger *slog.Logger) (*personabooster.Catalog, string, bool) {
	cleanDir := strings.TrimSpace(promptsDir)
	diskPath := ""
	if cleanDir != "" {
		cleanDir = filepath.Clean(cleanDir)
		diskPath = filepath.Join(cleanDir, personaPhaseDir, name+".yaml")
	}
	var probe personaPhaseCacheEntry
	if diskPath != "" {
		if info, err := os.Stat(diskPath); err == nil {
			probe.diskSeen, probe.diskSize, probe.diskMod = true, info.Size(), info.ModTime()
		}
	}
	key := cleanDir + "\x00" + name
	personaPhaseCacheMu.Lock()
	cached, hit := personaPhaseCache[key]
	personaPhaseCacheMu.Unlock()
	if hit && cached.diskSeen == probe.diskSeen && cached.diskSize == probe.diskSize && cached.diskMod.Equal(probe.diskMod) {
		return cached.catalog, cached.revision, cached.ok
	}
	probe.catalog, probe.revision, probe.ok = loadPersonaPhaseFile(diskPath, name, logger)
	personaPhaseCacheMu.Lock()
	if _, exists := personaPhaseCache[key]; !exists && len(personaPhaseCache) >= staticPromptCacheLimit {
		clear(personaPhaseCache)
	}
	personaPhaseCache[key] = probe
	personaPhaseCacheMu.Unlock()
	return probe.catalog, probe.revision, probe.ok
}

func loadPersonaPhaseFile(diskPath, name string, logger *slog.Logger) (*personabooster.Catalog, string, bool) {
	if diskPath != "" {
		if data, err := os.ReadFile(diskPath); err == nil {
			cat, parseErr := personabooster.ParseCatalog(data)
			if parseErr == nil && cat.Persona == name {
				return cat, "disk:" + personaPhaseDigest(data), true
			}
			if parseErr == nil {
				parseErr = fmt.Errorf("persona %q does not match the file name", cat.Persona)
			}
			logger.Warn("[PersonaBooster] Invalid persona phase catalog on disk; using the embedded catalog",
				"file", filepath.Base(diskPath), "error", parseErr)
		} else if !os.IsNotExist(err) {
			logger.Warn("[PersonaBooster] Unreadable persona phase catalog on disk",
				"file", filepath.Base(diskPath), "error", err)
		}
	}
	data, err := fs.ReadFile(promptsembed.FS, personaPhaseDir+"/"+name+".yaml")
	if err != nil {
		return nil, "", false
	}
	cat, err := personabooster.ParseCatalog(data)
	if err == nil && cat.Persona != name {
		err = fmt.Errorf("persona %q does not match the file name", cat.Persona)
	}
	if err != nil {
		logger.Error("[PersonaBooster] Embedded persona phase catalog is invalid", "file", name+".yaml", "error", err)
		return nil, "", false
	}
	return cat, "embed:" + personaPhaseDigest(data), true
}

func personaPhaseDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
