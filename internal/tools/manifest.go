package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Manifest manages the tool registry file (manifest.json).
type Manifest struct {
	mu       sync.RWMutex
	filePath string
}

// manifestFile is the on-disk schema for manifest.json.
// Version 1 was a bare map[string]string; version 2+ uses this wrapper.
type manifestFile struct {
	Version int               `json:"version"`
	Tools   map[string]string `json:"tools"`
}

const currentManifestVersion = 2

// NewManifest creates a manifest manager for the given tools directory.
func NewManifest(toolsDir string) *Manifest {
	return &Manifest{
		filePath: filepath.Join(toolsDir, "manifest.json"),
	}
}

// Load reads and returns the manifest contents (tool name → description).
func (m *Manifest) Load() (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	// Try versioned format first.
	var mf manifestFile
	if err := json.Unmarshal(data, &mf); err == nil && mf.Version > 0 {
		return mf.Tools, nil
	}

	// Fall back to legacy bare map (version 1).
	var legacy map[string]string
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}
	return legacy, nil
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
	return os.WriteFile(m.filePath, data, 0644)
}

// Register adds or updates a tool entry in the manifest.
func (m *Manifest) Register(name, description string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Read without lock (we already hold write lock)
	data, err := os.ReadFile(m.filePath)
	manifest := make(map[string]string)
	if err == nil {
		_ = json.Unmarshal(data, &manifest)
	}

	manifest[name] = description
	return m.save(manifest)
}

// SaveTool writes the Python code to a file and registers it in the manifest.
// Path traversal is blocked — name must be a plain filename without separators.
func (m *Manifest) SaveTool(toolsDir, name, description, code string) error {
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") || name == "" {
		return fmt.Errorf("invalid tool name: must be a simple filename without path separators")
	}
	toolPath := filepath.Join(toolsDir, name)
	absPath, _ := filepath.Abs(toolPath)
	absTools, _ := filepath.Abs(toolsDir)
	if !strings.HasPrefix(absPath, absTools) {
		return fmt.Errorf("path traversal blocked")
	}
	if err := os.WriteFile(toolPath, []byte(code), 0644); err != nil {
		return fmt.Errorf("failed to write tool file: %w", err)
	}
	return m.Register(name, description)
}
