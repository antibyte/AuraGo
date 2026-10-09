package server

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"

	"aurago/internal/config"
	"aurago/internal/localwiki"
	"aurago/internal/tools"

	"gopkg.in/yaml.v3"
)

// newLocalWikipediaManager builds the passive Local Wikipedia manager; Start
// binds it to the server lifetime, and its background loop loads the
// installed edition without delaying the server's startup. The manager builds
// its own HTTP client, which trusts only the Kiwix catalog host.
func newLocalWikipediaManager(cfg *config.Config, logger *slog.Logger) *localwiki.Manager {
	manager := localwiki.NewManager(localwiki.Deps{
		Logger:          logger,
		IsSensitivePath: localWikipediaSensitivePath(cfg.Directories.DataDir),
	})
	manager.Configure(localwiki.SettingsFromConfig(cfg))
	return manager
}

// syncLocalWikipediaSettings configures the manager from the current config
// snapshot. replaceConfigSnapshot calls it after every publication, so the
// config save, the setup wizard and every other publisher reach the manager;
// admin requests only read it. The snapshot is read and applied under
// localWikiSyncMu: a call that read an older snapshot finishes applying it
// before a later call reads the newer one, so publications that overlap can
// never leave the manager on older settings.
func (s *Server) syncLocalWikipediaSettings() {
	if s == nil || s.LocalWiki == nil {
		return
	}
	s.localWikiSyncMu.Lock()
	defer s.localWikiSyncMu.Unlock()
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return
	}
	settings := localwiki.SettingsFromConfig(cfg)
	if s.localWikiBeforeConfigure != nil {
		s.localWikiBeforeConfigure()
	}
	s.LocalWiki.Configure(settings)
}

// localWikipediaSensitivePath refuses AuraGo's data directory root, whose
// state.json belongs to AuraGo itself, and system locations (the Docker bind
// denylist via tools.IsSensitiveHostDirectory).
//
// Directories strictly below the data directory are allowed before the
// denylist: AuraGo already keeps all its data there, and common installs put
// it inside a refused tree (the install.sh root service in
// /root/aurago/data, /usr/local/aurago/data, C:\ProgramData\AuraGo\data,
// ~/Library/Application Support/aurago/data), which would otherwise refuse
// the default <data_dir>/wikipedia. The data directory counts in its absolute
// form and, when it exists and differs, in its resolved form, so the
// manager's second check of the resolved storage path (links, junctions, 8.3
// names) recognises it as well. A storage directory that is itself a link
// below the data directory is still refused when it resolves into a system
// location: the resolved path is no longer below the data directory.
//
// The exception is conservative: it needs a data directory at least two
// levels below its volume root (a data_dir of "/", "C:\" or "/usr" grants
// nothing), and a relative part whose components neither end in a dot or a
// space (Windows ignores those, so "data\. " is the data root) nor contain a
// colon (NTFS stream syntax). Such paths fall through to the denylist.
//
// The returned function does no I/O; the data directory is resolved once.
func localWikipediaSensitivePath(dataDir string) func(string) bool {
	roots := localWikipediaDataRoots(dataDir)
	return func(dir string) bool {
		for _, root := range roots {
			if sameLocalPath(dir, root) {
				return true
			}
		}
		for _, root := range roots {
			if localPathStrictlyBelow(dir, root) {
				return false
			}
		}
		return tools.IsSensitiveHostDirectory(dir)
	}
}

// localWikipediaDataRoots returns AuraGo's data directory as an absolute,
// cleaned path and, when it exists and resolves elsewhere, its resolved form.
func localWikipediaDataRoots(dataDir string) []string {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return nil
	}
	abs, err := filepath.Abs(dataDir)
	if err != nil {
		return []string{filepath.Clean(dataDir)}
	}
	roots := []string{abs}
	if resolved, err := localwiki.ResolveDirectory(abs); err == nil && filepath.IsAbs(resolved) && !sameLocalPath(resolved, abs) {
		roots = append(roots, filepath.Clean(resolved))
	}
	return roots
}

// sameLocalPath compares two paths lexically; Windows and macOS file systems
// are case-insensitive, so case is folded there.
func sameLocalPath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// localPathStrictlyBelow reports whether the absolute path dir lies strictly
// below the data directory root under the conservative rules described at
// localWikipediaSensitivePath. filepath.Rel folds case on Windows.
func localPathStrictlyBelow(dir, root string) bool {
	if strings.TrimSpace(dir) == "" || !filepath.IsAbs(dir) || !filepath.IsAbs(root) || localPathDepth(root) < 2 {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(dir))
	if err != nil || rel == "." || filepath.IsAbs(rel) {
		return false
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "" || part == "." || part == ".." || strings.ContainsRune(part, ':') ||
			strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
			return false
		}
	}
	return true
}

// localPathDepth counts the components of a cleaned absolute path below its
// volume root ("/" and "C:\" are 0, "/usr" is 1, "/root/aurago" is 2).
func localPathDepth(p string) int {
	p = filepath.Clean(p)
	rest := strings.Trim(p[len(filepath.VolumeName(p)):], string(filepath.Separator))
	if rest == "" {
		return 0
	}
	return len(strings.Split(rest, string(filepath.Separator)))
}

// validateLocalWikipediaSave validates the local_wikipedia values a config
// save changes, compared with config.yaml before the save (previous). Like
// the MQTT section (validateMQTTConfigPatch, checked only when a save touches
// it), a value the save leaves as it was is not checked again: the loader
// already replaced an unknown language or variant with its default, and the
// manager reports an unusable storage directory as data_dir_invalid. A
// hand-edited value that is (or has become) invalid therefore never blocks
// saving other sections, while every value a save sets is still checked.
func validateLocalWikipediaSave(s *Server, previous, candidate config.LocalWikipediaConfig, rt config.Runtime) error {
	changed := candidate
	if sameLocalWikipediaValue(previous.Language, candidate.Language) {
		changed.Language = ""
	}
	if sameLocalWikipediaValue(previous.Variant, candidate.Variant) {
		changed.Variant = ""
	}
	if err := config.ValidateLocalWikipediaConfig(changed); err != nil {
		return err
	}
	if strings.TrimSpace(previous.DataDir) == strings.TrimSpace(candidate.DataDir) {
		return nil
	}
	return validateLocalWikipediaSettings(s, candidate, rt)
}

func sameLocalWikipediaValue(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

// localWikipediaSectionFromRaw decodes the local_wikipedia section of a raw
// config.yaml map (a missing section is the zero value). The save handler
// reads it before merging the patch, as the baseline of
// validateLocalWikipediaSave.
func localWikipediaSectionFromRaw(raw map[string]interface{}) config.LocalWikipediaConfig {
	var section config.LocalWikipediaConfig
	data, err := yaml.Marshal(raw["local_wikipedia"])
	if err != nil {
		return section
	}
	if err := yaml.Unmarshal(data, &section); err != nil {
		return config.LocalWikipediaConfig{}
	}
	return section
}

// validateLocalWikipediaSettings checks a candidate storage directory on
// native installs: absolute, no system location, not AuraGo's data root
// (directories below it are fine, see localWikipediaSensitivePath).
// Docker ignores the field because the directory is fixed in the data volume.
func validateLocalWikipediaSettings(s *Server, candidate config.LocalWikipediaConfig, rt config.Runtime) error {
	if rt.IsDocker {
		return nil
	}
	dir := strings.TrimSpace(candidate.DataDir)
	if dir == "" {
		return nil
	}
	if !filepath.IsAbs(dir) {
		return fmt.Errorf("local_wikipedia.data_dir must be an absolute path")
	}
	dataDir := ""
	if current := s.ConfigSnapshot(); current != nil {
		dataDir = current.Directories.DataDir
	}
	if localWikipediaSensitivePath(dataDir)(dir) {
		return fmt.Errorf("local_wikipedia.data_dir must not be a system directory or AuraGo's data directory")
	}
	return nil
}

// injectLocalWikipediaDefaults shows the loader defaults for keys an older
// config.yaml does not contain yet (agent_access and update_check default on).
func injectLocalWikipediaDefaults(rawCfg map[string]interface{}, cfg *config.Config) {
	if cfg == nil {
		return
	}
	section, ok := rawCfg["local_wikipedia"].(map[string]interface{})
	if !ok {
		section = make(map[string]interface{})
		rawCfg["local_wikipedia"] = section
	}
	setDefaultBool(section, "enabled", cfg.LocalWikipedia.Enabled)
	setDefaultBool(section, "agent_access", cfg.LocalWikipedia.AgentAccess)
	setDefaultBool(section, "update_check", cfg.LocalWikipedia.UpdateCheck)
	for key, value := range map[string]string{
		"language": cfg.LocalWikipedia.Language,
		"variant":  cfg.LocalWikipedia.Variant,
		"data_dir": cfg.LocalWikipedia.DataDir,
	} {
		if _, exists := section[key]; !exists {
			section[key] = value
		}
	}
}
