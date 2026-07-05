package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestParseSkillSpectorReportMapsRecommendations(t *testing.T) {
	tests := []struct {
		name       string
		json       string
		wantStatus SecurityStatus
		wantScore  float64
	}{
		{
			name:       "safe",
			json:       `{"risk_assessment":{"score":0,"severity":"LOW","recommendation":"SAFE"},"metadata":{"skillspector_version":"2.0.0","llm_requested":true,"llm_available":true,"llm_used":true,"scan_mode":"static"}}`,
			wantStatus: SecurityClean,
			wantScore:  0,
		},
		{
			name:       "caution",
			json:       `{"risk_assessment":{"score":35,"severity":"MEDIUM","recommendation":"CAUTION"},"issues":[{"id":"SC1","category":"supply_chain","severity":"LOW","confidence":0.8,"location":{"file":"SKILL.md","start_line":4}}],"metadata":{"skillspector_version":"2.0.0","llm_requested":false}}`,
			wantStatus: SecurityWarning,
			wantScore:  35,
		},
		{
			name:       "do_not_install",
			json:       `{"risk_assessment":{"score":78,"severity":"HIGH","recommendation":"DO_NOT_INSTALL"},"issues":[{"id":"E2","category":"data_exfiltration","severity":"HIGH","confidence":0.94,"location":{"file":"scripts/run.py","start_line":12},"message":"env harvesting"}],"metadata":{"skillspector_version":"2.0.0","scan_mode":"static"}}`,
			wantStatus: SecurityDangerous,
			wantScore:  78,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, status, err := parseSkillSpectorReport([]byte(tt.json))
			if err != nil {
				t.Fatalf("parseSkillSpectorReport returned error: %v", err)
			}
			if status != tt.wantStatus {
				t.Fatalf("status=%s, want %s", status, tt.wantStatus)
			}
			if report.Score != tt.wantScore {
				t.Fatalf("score=%v, want %v", report.Score, tt.wantScore)
			}
			if report.Recommendation == "" {
				t.Fatal("expected recommendation to be populated")
			}
			if report.LLMUsed {
				t.Fatal("llm_used must stay false because AuraGo runs SkillSpector with --no-llm")
			}
		})
	}
}

func TestParseSkillSpectorReportRejectsInvalidJSON(t *testing.T) {
	_, status, err := parseSkillSpectorReport([]byte(`not json`))
	if err == nil {
		t.Fatal("expected parse error")
	}
	if status != SecurityError {
		t.Fatalf("status=%s, want %s", status, SecurityError)
	}
}

func TestDetermineSecurityStatusIncludesSkillSpector(t *testing.T) {
	tests := []struct {
		name string
		rep  *SecurityReport
		want SecurityStatus
	}{
		{
			name: "caution becomes warning",
			rep: &SecurityReport{
				SkillSpector: &SkillSpectorReport{Recommendation: "CAUTION", Severity: "MEDIUM", Score: 35},
			},
			want: SecurityWarning,
		},
		{
			name: "do not install becomes dangerous",
			rep: &SecurityReport{
				SkillSpector: &SkillSpectorReport{Recommendation: "DO_NOT_INSTALL", Severity: "HIGH", Score: 78},
			},
			want: SecurityDangerous,
		},
		{
			name: "scanner error becomes security error",
			rep: &SecurityReport{
				SkillSpector: &SkillSpectorReport{Error: "skillspector not found"},
			},
			want: SecurityError,
		},
		{
			name: "static critical still blocks",
			rep: &SecurityReport{
				StaticAnalysis: []Finding{{Severity: "critical", Category: "exec", Message: "exec"}},
				SkillSpector:   &SkillSpectorReport{Recommendation: "SAFE", Severity: "LOW", Score: 0},
			},
			want: SecurityDangerous,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetermineSecurityStatus(tt.rep); got != tt.want {
				t.Fatalf("DetermineSecurityStatus=%s, want %s", got, tt.want)
			}
		})
	}
}

func TestRunSkillSpectorScanMissingBinaryReturnsSecurityError(t *testing.T) {
	report, status, err := RunSkillSpectorScan(context.Background(), t.TempDir(), SkillSpectorConfig{
		Enabled:        true,
		CommandPath:    "definitely-missing-skillspector-binary",
		Timeout:        time.Second,
		MaxOutputBytes: 1024,
	})
	if err == nil {
		t.Fatal("expected missing binary error")
	}
	if status != SecurityError {
		t.Fatalf("status=%s, want %s", status, SecurityError)
	}
	if report == nil || !strings.Contains(strings.ToLower(report.Error), "skillspector") {
		t.Fatalf("report=%+v, want skillspector context", report)
	}
}

func TestRunSkillSpectorScanParsesJSONWithStderrNoise(t *testing.T) {
	fake := buildFakeSkillSpector(t)
	t.Setenv("SKILLSPECTOR_FAKE_REPORT", `{"risk_assessment":{"score":0,"severity":"LOW","recommendation":"SAFE"},"metadata":{"skillspector_version":"2.0.0","scan_mode":"static"}}`)
	t.Setenv("SKILLSPECTOR_FAKE_STDERR", "diagnostic warning\n")
	t.Setenv("SKILLSPECTOR_FAKE_EXIT_CODE", "0")

	report, status, err := RunSkillSpectorScan(context.Background(), t.TempDir(), SkillSpectorConfig{
		Enabled:        true,
		CommandPath:    fake,
		Timeout:        5 * time.Second,
		MaxOutputBytes: 64 * 1024,
	})
	if err != nil {
		t.Fatalf("RunSkillSpectorScan returned error: %v", err)
	}
	if status != SecurityClean {
		t.Fatalf("status=%s, want %s", status, SecurityClean)
	}
	if report == nil || report.Recommendation != "SAFE" {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestScanSkillUsesSkillSpectorExitOneAsDangerousFinding(t *testing.T) {
	mgr, _ := setupTestSkillManager(t)
	entry, err := mgr.CreateSkillEntry("ss_block", "SkillSpector block test", `def run(): return "ok"`, SkillTypeUser, "test", "", nil)
	if err != nil {
		t.Fatalf("CreateSkillEntry: %v", err)
	}
	fake := buildFakeSkillSpector(t)
	t.Setenv("SKILLSPECTOR_FAKE_REPORT", `{"risk_assessment":{"score":78,"severity":"HIGH","recommendation":"DO_NOT_INSTALL"},"issues":[{"id":"E2","category":"data_exfiltration","severity":"HIGH","confidence":0.94,"location":{"file":"ss_block.py","start_line":1},"message":"blocked"}],"metadata":{"skillspector_version":"2.0.0","llm_requested":false,"scan_mode":"static"}}`)
	t.Setenv("SKILLSPECTOR_FAKE_EXIT_CODE", "1")

	report, status, err := mgr.ScanSkill(context.Background(), entry.ID, "", nil, false, false, SkillSpectorConfig{
		Enabled:        true,
		CommandPath:    fake,
		Timeout:        5 * time.Second,
		MaxOutputBytes: 64 * 1024,
	})
	if err != nil {
		t.Fatalf("ScanSkill returned error for exit-code-1 finding: %v", err)
	}
	if status != SecurityDangerous {
		t.Fatalf("status=%s, want %s", status, SecurityDangerous)
	}
	if report.SkillSpector == nil || report.SkillSpector.Recommendation != "DO_NOT_INSTALL" || len(report.SkillSpector.Findings) != 1 {
		t.Fatalf("unexpected SkillSpector report: %+v", report.SkillSpector)
	}
	got, err := mgr.GetSkill(entry.ID)
	if err != nil {
		t.Fatalf("GetSkill: %v", err)
	}
	if got.SecurityStatus != SecurityDangerous {
		t.Fatalf("persisted security status=%s, want %s", got.SecurityStatus, SecurityDangerous)
	}
}

func buildFakeSkillSpector(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	code := `package main

import (
	"fmt"
	"os"
	"strconv"
)

func main() {
	fmt.Print(os.Getenv("SKILLSPECTOR_FAKE_REPORT"))
	fmt.Fprint(os.Stderr, os.Getenv("SKILLSPECTOR_FAKE_STDERR"))
	code, _ := strconv.Atoi(os.Getenv("SKILLSPECTOR_FAKE_EXIT_CODE"))
	os.Exit(code)
}
`
	if err := os.WriteFile(src, []byte(code), 0o600); err != nil {
		t.Fatalf("write fake skillspector source: %v", err)
	}
	exeName := "skillspector-fake"
	if runtime.GOOS == "windows" {
		exeName += ".exe"
	}
	exePath := filepath.Join(dir, exeName)
	cmd := exec.Command("go", "build", "-o", exePath, src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake skillspector: %v\n%s", err, out)
	}
	return exePath
}
