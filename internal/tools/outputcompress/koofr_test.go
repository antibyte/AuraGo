package outputcompress

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestCompressKoofrList_LargeDirectory(t *testing.T) {
	files := make([]map[string]interface{}, 0, 64)
	files = append(files, map[string]interface{}{
		"name":        "music",
		"type":        "dir",
		"size":        0,
		"contentType": "",
		"modified":    int64(1777496258153),
		"tags":        map[string]interface{}{},
	})
	for i := 0; i < 63; i++ {
		files = append(files, map[string]interface{}{
			"name":        fmt.Sprintf("song_%02d.mp3", i),
			"type":        "file",
			"size":        5_001_000 + i,
			"contentType": "audio/mpeg",
			"modified":    int64(1777578428741 + i),
			"hash":        fmt.Sprintf("hash-%02d-verbose-noise", i),
			"tags":        map[string]interface{}{"mood": "dark", "genre": "synth"},
		})
	}
	data, err := json.Marshal(map[string]interface{}{
		"status": "success",
		"response": map[string]interface{}{
			"files": files,
		},
	})
	if err != nil {
		t.Fatalf("marshal koofr output: %v", err)
	}

	input := "Tool Output: " + string(data)
	result, filter := compressAPIOutput("koofr", input)

	if filter != "koofr-list" {
		t.Fatalf("filter = %q, want koofr-list", filter)
	}
	if !strings.Contains(result, "Koofr list: 64 items") {
		t.Fatalf("expected item count summary, got:\n%s", result)
	}
	if !strings.Contains(result, "1 dirs, 63 files") {
		t.Fatalf("expected dir/file summary, got:\n%s", result)
	}
	if !strings.Contains(result, "music/") {
		t.Fatalf("expected directory entry, got:\n%s", result)
	}
	if !strings.Contains(result, "song_00.mp3") {
		t.Fatalf("expected first file entry, got:\n%s", result)
	}
	if !strings.Contains(result, "+ 39 more files") {
		t.Fatalf("expected omitted file count, got:\n%s", result)
	}
	if strings.Contains(result, "hash-") || strings.Contains(result, "tags") {
		t.Fatalf("compressed output leaked noisy fields:\n%s", result)
	}
	if len(result) >= len(input) {
		t.Fatalf("expected compression, got result len %d >= input len %d", len(result), len(input))
	}
}

func TestCompressRoutesKoofrThroughAPICompression(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinChars = 1
	output := `Tool Output: {"status":"success","response":{"files":[{"name":"pictures","type":"dir","size":0}]}}`

	result, stats := Compress("koofr", "", output, cfg)

	if stats.FilterUsed != "koofr-list" {
		t.Fatalf("FilterUsed = %q, want koofr-list", stats.FilterUsed)
	}
	if !strings.Contains(result, "Koofr list: 1 items") {
		t.Fatalf("expected Koofr compact output, got %q", result)
	}
}
