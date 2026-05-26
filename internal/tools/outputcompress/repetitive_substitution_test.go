package outputcompress

import (
	"fmt"
	"strings"
	"testing"
)

func repeatedBackupLogLines(count int) string {
	var b strings.Builder
	for i := 0; i < count; i++ {
		fmt.Fprintf(&b, "2026-05-26T21:%02d:00Z INFO service=backup component=scheduler message=processed recurring snapshot job id=%03d status=ok\n", i%60, i)
	}
	return b.String()
}

func TestDefaultConfig_AdvancedCompressionDisabledByDefault(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.RepetitiveSubstitution.Enabled {
		t.Fatal("repetitive substitution must be disabled by default")
	}
	if !cfg.RepetitiveSubstitution.LZWEnabled {
		t.Fatal("LZW-style substitution should be ready when the parent toggle is enabled")
	}
	if cfg.RepetitiveSubstitution.LTSCLiteEnabled {
		t.Fatal("LTSC-lite must stay disabled by default")
	}
	if cfg.TOONJSON.Enabled {
		t.Fatal("TOON JSON conversion must be disabled by default")
	}
}

func TestCompress_RepetitiveSubstitutionCompressesLogLikeShellOutput(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinChars = 10
	cfg.RepetitiveSubstitution.Enabled = true
	cfg.RepetitiveSubstitution.MinSavingsPercent = 10

	output := repeatedBackupLogLines(80)
	result, stats := Compress("execute_shell", "docker logs backup", output, cfg)

	if !strings.Contains(stats.FilterUsed, "repetitive-substitution") {
		t.Fatalf("filter = %q, want repetitive-substitution suffix", stats.FilterUsed)
	}
	if !strings.Contains(result, "[repetitive-substitutions]") {
		t.Fatalf("compressed output missing substitution dictionary:\n%s", result)
	}
	if len(result) >= len(output) {
		t.Fatalf("result length = %d, want shorter than raw %d", len(result), len(output))
	}
	if strings.Count(result, "INFO service=backup component=scheduler message=processed recurring snapshot job") > 2 {
		t.Fatalf("repeated phrase still appears too often in result:\n%s", result)
	}
}

func TestCompress_RepetitiveSubstitutionHonorsDisabledLZW(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinChars = 10
	cfg.RepetitiveSubstitution.Enabled = true
	cfg.RepetitiveSubstitution.LZWEnabled = false
	cfg.RepetitiveSubstitution.MinSavingsPercent = 10

	_, stats := Compress("execute_shell", "docker logs backup", repeatedBackupLogLines(80), cfg)
	if strings.Contains(stats.FilterUsed, "repetitive-substitution") {
		t.Fatalf("filter = %q, want repetitive substitution disabled", stats.FilterUsed)
	}
}

func TestCompress_RepetitiveSubstitutionSkipsExactCopySensitiveOutputs(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinChars = 10
	cfg.RepetitiveSubstitution.Enabled = true

	cases := []struct {
		name    string
		tool    string
		command string
		output  string
	}{
		{
			name:    "go source read",
			tool:    "execute_shell",
			command: "cat main.go",
			output:  strings.Repeat("package main\n\nfunc handleRequest() {\n\tfmt.Println(\"common phrase repeated in source\")\n}\n", 20),
		},
		{
			name:    "git diff",
			tool:    "execute_shell",
			command: "git diff",
			output:  strings.Repeat("diff --git a/main.go b/main.go\n@@ -1,3 +1,3 @@\n-common phrase repeated in diff\n+common phrase repeated in diff\n", 20),
		},
		{
			name:    "json config",
			tool:    "execute_shell",
			command: "cat config.json",
			output:  `{"services":[{"name":"a","description":"common phrase repeated in json config"},{"name":"b","description":"common phrase repeated in json config"},{"name":"c","description":"common phrase repeated in json config"}]}`,
		},
		{
			name:    "file reader content",
			tool:    "file_reader_advanced",
			command: "",
			output:  strings.Repeat("common phrase repeated in file content\n", 30),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, stats := Compress(tc.tool, tc.command, tc.output, cfg)
			if strings.Contains(stats.FilterUsed, "repetitive-substitution") {
				t.Fatalf("filter = %q, want repetitive substitution skipped", stats.FilterUsed)
			}
		})
	}
}

func TestCompress_RepetitiveSubstitutionSkipsErrorsAndOversizedInput(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinChars = 10
	cfg.RepetitiveSubstitution.Enabled = true
	cfg.RepetitiveSubstitution.MaxInputChars = 80

	errorOutput := strings.Repeat("[EXECUTION ERROR] recurring failure from backup worker\n", 20)
	_, errorStats := Compress("execute_shell", "docker logs backup", errorOutput, cfg)
	if strings.Contains(errorStats.FilterUsed, "repetitive-substitution") {
		t.Fatalf("error filter = %q, want repetitive substitution skipped", errorStats.FilterUsed)
	}

	oversized := repeatedBackupLogLines(30)
	_, oversizedStats := Compress("execute_shell", "docker logs backup", oversized, cfg)
	if strings.Contains(oversizedStats.FilterUsed, "repetitive-substitution") {
		t.Fatalf("oversized filter = %q, want repetitive substitution skipped", oversizedStats.FilterUsed)
	}
}

func TestCompress_RepetitiveSubstitutionAvoidsMarkerCollisions(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinChars = 10
	cfg.RepetitiveSubstitution.Enabled = true
	cfg.RepetitiveSubstitution.MinSavingsPercent = 10

	output := "@@OTK1_1@@ appears in the original output and must not be reused\n" + repeatedBackupLogLines(80)
	result, stats := Compress("execute_shell", "docker logs backup", output, cfg)
	if !strings.Contains(stats.FilterUsed, "repetitive-substitution") {
		t.Fatalf("filter = %q, want repetitive substitution", stats.FilterUsed)
	}
	if strings.Contains(result, "@@OTK1_1@@ =") {
		t.Fatalf("dictionary reused a marker already present in the input:\n%s", result)
	}
}

func TestCompress_TOONJSONOnlyForKnownStructuredAPIOutputs(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinChars = 10
	cfg.TOONJSON.Enabled = true
	cfg.TOONJSON.MinSavingsPercent = 5

	output := `[
{"id":"abc123456789","name":"api","status":"running","image":"aurago/api:latest"},
{"id":"def123456789","name":"worker","status":"running","image":"aurago/worker:latest"},
{"id":"ghi123456789","name":"db","status":"healthy","image":"postgres:16"}
]`

	result, stats := Compress("docker", "", output, cfg)
	if !strings.Contains(stats.FilterUsed, "toon-json") {
		t.Fatalf("filter = %q, want toon-json", stats.FilterUsed)
	}
	if !strings.Contains(result, "[toon-json rows=3") {
		t.Fatalf("TOON output missing header:\n%s", result)
	}

	apiResult, apiStats := Compress("api_request", "", output, cfg)
	if strings.Contains(apiStats.FilterUsed, "toon-json") {
		t.Fatalf("api_request filter = %q, want TOON skipped", apiStats.FilterUsed)
	}
	if strings.Contains(apiResult, "[toon-json") {
		t.Fatalf("api_request unexpectedly converted to TOON:\n%s", apiResult)
	}
}

func TestCompress_ConservativeRollbackReturnsOriginalWhenStageExpands(t *testing.T) {
	cfg := Config{
		Enabled:           true,
		MinChars:          10,
		PreserveErrors:    true,
		APICompression:    true,
		TOONJSON:          TOONJSONConfig{Enabled: true, MinSavingsPercent: 99},
		ShellCompression:  true,
		PythonCompression: true,
	}
	output := `[{"a":1},{"a":2},{"a":3}]`

	result, stats := Compress("docker", "", output, cfg)
	if result != output {
		t.Fatalf("result = %q, want original after conservative rollback", result)
	}
	if stats.FilterUsed != "skipped-expanded" {
		t.Fatalf("filter = %q, want skipped-expanded", stats.FilterUsed)
	}
	if stats.Ratio != 1.0 {
		t.Fatalf("ratio = %f, want 1.0", stats.Ratio)
	}
}
