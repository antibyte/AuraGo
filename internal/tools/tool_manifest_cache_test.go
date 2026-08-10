package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestToolManifestCachesDefensiveSnapshotAndGeneration(t *testing.T) {
	dir := t.TempDir()
	manifest := NewManifest(dir)
	first, err := manifest.Load()
	if err != nil {
		t.Fatalf("Load empty manifest: %v", err)
	}
	first["caller-mutation"] = "must not leak"
	second, err := manifest.Load()
	if err != nil {
		t.Fatalf("Load cached manifest: %v", err)
	}
	if _, leaked := second["caller-mutation"]; leaked {
		t.Fatal("manifest cache returned an aliased map")
	}
	initialGeneration := manifest.Generation()
	if err := manifest.Register("local.py", "local tool"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if manifest.Generation() <= initialGeneration {
		t.Fatal("AuraGo-owned manifest mutation did not advance generation")
	}
	loaded, err := manifest.Load()
	if err != nil || loaded["local.py"] != "local tool" {
		t.Fatalf("Load after Register = %v, %v", loaded, err)
	}

	external := manifestFile{Version: currentManifestVersion, Tools: map[string]string{"external.py": "external tool"}}
	raw, err := json.Marshal(external)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0o600); err != nil {
		t.Fatalf("external manifest write: %v", err)
	}
	manifest.mu.Lock()
	manifest.checked = time.Now().Add(-manifestSnapshotTTL - time.Second)
	manifest.mu.Unlock()
	beforeExternal := manifest.Generation()
	loaded, err = manifest.Load()
	if err != nil {
		t.Fatalf("Load external revision: %v", err)
	}
	if loaded["external.py"] != "external tool" || manifest.Generation() <= beforeExternal {
		t.Fatalf("external snapshot not observed: entries=%v generation=%d", loaded, manifest.Generation())
	}
}
