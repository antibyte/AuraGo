package agent

import (
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/config"

	openai "github.com/sashabaranov/go-openai"
)

func toolOperationNames(t *testing.T, schemas []openai.Tool, name string) []string {
	t.Helper()
	for _, schema := range schemas {
		if schema.Function == nil || schema.Function.Name != name {
			continue
		}
		parameters, ok := schema.Function.Parameters.(map[string]interface{})
		if !ok {
			t.Fatalf("%s parameters type = %T", name, schema.Function.Parameters)
		}
		properties, ok := parameters["properties"].(map[string]interface{})
		if !ok {
			t.Fatalf("%s properties missing", name)
		}
		operation, ok := properties["operation"].(map[string]interface{})
		if !ok {
			t.Fatalf("%s operation missing", name)
		}
		switch values := operation["enum"].(type) {
		case []string:
			return append([]string(nil), values...)
		case []interface{}:
			result := make([]string, 0, len(values))
			for _, value := range values {
				result = append(result, value.(string))
			}
			return result
		default:
			t.Fatalf("%s operation enum type = %T", name, operation["enum"])
		}
	}
	return nil
}

func containsOperation(operations []string, wanted string) bool {
	for _, operation := range operations {
		if operation == wanted {
			return true
		}
	}
	return false
}

func TestBluetoothToolSchemaCapabilityGates(t *testing.T) {
	cfg := &config.Config{}
	cfg.Bluetooth.Enabled = true
	cfg.Bluetooth.ReadOnly = true

	if names := builtinToolNames(buildToolFlagsFromConfig(cfg)); containsOperation(names, "bluetooth") {
		t.Fatalf("bluetooth tool present without a usable runtime adapter: %v", names)
	}

	cfg.Runtime.Bluetooth.Usable = true
	readOnlyOps := toolOperationNames(t, builtinToolSchemas(buildToolFlagsFromConfig(cfg)), "bluetooth")
	for _, operation := range []string{"status", "list", "discover"} {
		if !containsOperation(readOnlyOps, operation) {
			t.Fatalf("read-only operations = %v, missing %s", readOnlyOps, operation)
		}
	}
	for _, operation := range []string{"pair", "connect", "disconnect", "play", "speak", "stop"} {
		if containsOperation(readOnlyOps, operation) {
			t.Fatalf("read-only operations unexpectedly contain %s: %v", operation, readOnlyOps)
		}
	}

	cfg.Bluetooth.ReadOnly = false
	writeOps := toolOperationNames(t, builtinToolSchemas(buildToolFlagsFromConfig(cfg)), "bluetooth")
	for _, operation := range []string{"pair", "connect", "disconnect"} {
		if !containsOperation(writeOps, operation) {
			t.Fatalf("write operations = %v, missing %s", writeOps, operation)
		}
	}

	cfg.Bluetooth.AllowPlayback = true
	cfg.Runtime.Bluetooth.Audio.Usable = true
	cfg.Runtime.Bluetooth.Audio.Backend = "pipewire"
	audioOps := toolOperationNames(t, builtinToolSchemas(buildToolFlagsFromConfig(cfg)), "bluetooth")
	for _, operation := range []string{"play", "speak", "playback_status", "stop"} {
		if !containsOperation(audioOps, operation) {
			t.Fatalf("audio operations = %v, missing %s", audioOps, operation)
		}
	}
}

func TestBluetoothFeatureFlagsChangeSchemaCacheKey(t *testing.T) {
	base := ToolFeatureFlags{BluetoothEnabled: true}
	write := base
	write.BluetoothWriteEnabled = true
	audio := write
	audio.BluetoothAudioEnabled = true
	if base.Key() == write.Key() || write.Key() == audio.Key() || base.Key() == audio.Key() {
		t.Fatalf("Bluetooth permission variants must have distinct schema cache keys")
	}
}

func TestResolveBluetoothPlaybackSourceRequiresOneSafeLocalSource(t *testing.T) {
	workspace := t.TempDir()
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = workspace
	cfg.Directories.DataDir = t.TempDir()
	source := filepath.Join(workspace, "song.mp3")
	if err := os.WriteFile(source, []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}

	resolved, err := resolveBluetoothPlaybackSource(cfg, nil, bluetoothToolArgs{LocalPath: "song.mp3"})
	if err != nil {
		t.Fatalf("resolve local source: %v", err)
	}
	if resolved != source {
		t.Fatalf("resolved source = %q, want %q", resolved, source)
	}
	if _, err := resolveBluetoothPlaybackSource(cfg, nil, bluetoothToolArgs{}); err == nil {
		t.Fatal("expected missing source error")
	}
	if _, err := resolveBluetoothPlaybackSource(cfg, nil, bluetoothToolArgs{LocalPath: "song.mp3", MediaID: 1}); err == nil {
		t.Fatal("expected mutually exclusive source error")
	}
	if _, err := resolveBluetoothPlaybackSource(cfg, nil, bluetoothToolArgs{LocalPath: "../outside.mp3"}); err == nil {
		t.Fatal("expected path traversal error")
	}
	dataSource := filepath.Join(cfg.Directories.DataDir, "generated.mp3")
	if err := os.WriteFile(dataSource, []byte("generated audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolved, err = resolveBluetoothPlaybackSource(cfg, nil, bluetoothToolArgs{LocalPath: "data/generated.mp3"})
	if err != nil {
		t.Fatalf("resolve data source: %v", err)
	}
	if resolved != dataSource {
		t.Fatalf("resolved data source = %q, want %q", resolved, dataSource)
	}
}
