package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Manifest manages the tool registry file (manifest.json).
type Manifest struct {
	mu           sync.RWMutex
	filePath     string
	cached       map[string]string
	cacheReady   bool
	checked      time.Time
	fileRevision string
	generation   uint64
}

// manifestFile is the on-disk schema for manifest.json.
// Version 1 was a bare map[string]string; version 2+ uses this wrapper.
type manifestFile struct {
	Version int               `json:"version"`
	Tools   map[string]string `json:"tools"`
}

const currentManifestVersion = 2
const manifestSnapshotTTL = 5 * time.Second

// NewManifest creates a manifest manager for the given tools directory.
func NewManifest(toolsDir string) *Manifest {
	return &Manifest{
		filePath: filepath.Join(toolsDir, "manifest.json"),
	}
}

// Load reads and returns the manifest contents (tool name → description).
func (m *Manifest) Load() (map[string]string, error) {
	if m == nil {
		return map[string]string{}, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	if m.cacheReady && now.Sub(m.checked) < manifestSnapshotTTL {
		return cloneManifestEntries(m.cached), nil
	}
	revision := manifestFileRevision(m.filePath)
	if m.cacheReady && revision == m.fileRevision {
		m.checked = now
		return cloneManifestEntries(m.cached), nil
	}

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			m.updateSnapshotLocked(map[string]string{}, revision, now)
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}
	entries, err := parseManifestData(data)
	if err != nil {
		return nil, err
	}
	m.updateSnapshotLocked(entries, revision, now)
	return cloneManifestEntries(entries), nil
}

func parseManifestData(data []byte) (map[string]string, error) {
	// Try versioned format first.
	var mf manifestFile
	if err := json.Unmarshal(data, &mf); err == nil && mf.Version > 0 {
		if mf.Tools == nil {
			return map[string]string{}, nil
		}
		return mf.Tools, nil
	}

	// Fall back to legacy bare map (version 1).
	var legacy map[string]string
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}
	return legacy, nil
}

func (m *Manifest) updateSnapshotLocked(entries map[string]string, revision string, now time.Time) {
	changed := !m.cacheReady || revision != m.fileRevision
	m.cached = cloneManifestEntries(entries)
	m.cacheReady = true
	m.checked = now
	m.fileRevision = revision
	if changed {
		m.generation++
	}
}

func manifestFileRevision(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "missing"
		}
		return "error:" + err.Error()
	}
	return fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UnixNano())
}

func cloneManifestEntries(entries map[string]string) map[string]string {
	out := make(map[string]string, len(entries))
	for name, description := range entries {
		out[name] = description
	}
	return out
}

// Generation changes whenever the cached manifest snapshot observes a new
// on-disk revision or AuraGo writes the manifest itself.
func (m *Manifest) Generation() uint64 {
	if m == nil {
		return 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.generation
}

// Invalidate forces the next Load to re-check the on-disk manifest.
func (m *Manifest) Invalidate() {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.checked = time.Time{}
	m.fileRevision = ""
	m.cacheReady = false
	m.generation++
	m.mu.Unlock()
}

// Save writes the manifest to disk in the versioned format.
func (m *Manifest) save(tools map[string]string) error {
	mf := manifestFile{
		Version: currentManifestVersion,
		Tools:   tools,
	}
	data, err := json.MarshalIndent(mf, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}
	// Atomic write: temp file + rename prevents manifest corruption on crash
	tmp := m.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("write temp manifest: %w", err)
	}
	if err := os.Rename(tmp, m.filePath); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename temp manifest: %w", err)
	}
	return nil
}

// Register adds or updates a tool entry in the manifest.
func (m *Manifest) Register(name, description string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	manifest := make(map[string]string)
	data, err := os.ReadFile(m.filePath)
	if err == nil {
		if err := unmarshalManifestRegisterData(data, &manifest); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read manifest for register: %w", err)
	}

	manifest[name] = description
	if err := m.save(manifest); err != nil {
		return err
	}
	m.cached = cloneManifestEntries(manifest)
	m.cacheReady = true
	m.checked = time.Now()
	m.fileRevision = manifestFileRevision(m.filePath)
	m.generation++
	return nil
}

func unmarshalManifestRegisterData(data []byte, manifest *map[string]string) error {
	if len(data) == 0 {
		return nil
	}
	var versioned manifestFile
	if err := json.Unmarshal(data, &versioned); err == nil && versioned.Version > 0 {
		if versioned.Tools == nil {
			*manifest = map[string]string{}
		} else {
			*manifest = versioned.Tools
		}
		return nil
	}
	var legacy map[string]string
	if err := json.Unmarshal(data, &legacy); err != nil {
		return fmt.Errorf("failed to parse manifest for register: %w", err)
	}
	*manifest = legacy
	return nil
}

// SaveTool writes the Python code to a file and registers it in the manifest.
// Path traversal is blocked — name must be a plain filename without separators.
func (m *Manifest) SaveTool(toolsDir, name, description, code string) error {
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") || name == "" {
		return fmt.Errorf("invalid tool name: must be a simple filename without path separators")
	}
	toolPath := filepath.Join(toolsDir, name)
	absPath, err := filepath.Abs(toolPath)
	if err != nil {
		return fmt.Errorf("failed to resolve tool path: %w", err)
	}
	absTools, err := filepath.Abs(toolsDir)
	if err != nil {
		return fmt.Errorf("failed to resolve tools directory: %w", err)
	}
	if !strings.HasPrefix(absPath, absTools+string(filepath.Separator)) {
		return fmt.Errorf("path traversal blocked")
	}
	if err := os.WriteFile(toolPath, []byte(code), 0600); err != nil {
		return fmt.Errorf("failed to write tool file: %w", err)
	}
	return m.Register(name, description)
}
