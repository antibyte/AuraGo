package tools

import (
	"testing"
)

// TestStaticCodeAnalysis_Clean verifies clean code produces no findings.
func TestStaticCodeAnalysis_Clean(t *testing.T) {
	code := `import json
import os

def process(data):
    result = json.loads(data)
    return result
`
	findings := StaticCodeAnalysis(code)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for clean code, got %d: %v", len(findings), findings)
	}
}

// TestStaticCodeAnalysis_EvalExec detects eval and exec usage.
func TestStaticCodeAnalysis_EvalExec(t *testing.T) {
	code := `result = eval(user_input)
exec(code_string)
`
	findings := StaticCodeAnalysis(code)
	if len(findings) < 2 {
		t.Fatalf("expected at least 2 findings, got %d", len(findings))
	}
	categories := map[string]bool{}
	for _, f := range findings {
		categories[f.Pattern] = true
	}
	if !categories["eval_usage"] {
		t.Error("missing eval_usage finding")
	}
	if !categories["exec_usage"] {
		t.Error("missing exec_usage finding")
	}
}

// TestStaticCodeAnalysis_Critical detects critical patterns.
func TestStaticCodeAnalysis_Critical(t *testing.T) {
	code := `import subprocess
subprocess.run("rm -rf /", shell=True)
import pickle
data = pickle.load(open("data.pkl", "rb"))
`
	findings := StaticCodeAnalysis(code)
	criticalCount := 0
	for _, f := range findings {
		if f.Severity == "critical" {
			criticalCount++
		}
	}
	if criticalCount < 2 {
		t.Errorf("expected at least 2 critical findings, got %d", criticalCount)
	}
}

// TestStaticCodeAnalysis_LineNumbers verifies line numbers are correct.
func TestStaticCodeAnalysis_LineNumbers(t *testing.T) {
	code := `# line 1
# line 2
result = eval("1+1")
`
	findings := StaticCodeAnalysis(code)
	if len(findings) == 0 {
		t.Fatal("expected at least 1 finding")
	}
	if findings[0].Line != 3 {
		t.Errorf("expected finding on line 3, got line %d", findings[0].Line)
	}
}

// TestValidateSkillUpload_Valid accepts valid Python files.
func TestValidateSkillUpload_Valid(t *testing.T) {
	code := []byte(`import json
def run():
    return {"result": "ok"}
`)
	result := ValidateSkillUpload(code, "my_skill.py", 1)
	if !result.Passed {
		t.Errorf("expected valid upload to pass, findings: %v", result.Findings)
	}
}

// TestValidateSkillUpload_WrongExtension rejects non-.py files.
func TestValidateSkillUpload_WrongExtension(t *testing.T) {
	result := ValidateSkillUpload([]byte("data"), "script.sh", 1)
	if result.Passed {
		t.Error("expected non-.py file to be rejected")
	}
}

// TestValidateSkillUpload_TooLarge rejects oversized files.
func TestValidateSkillUpload_TooLarge(t *testing.T) {
	data := make([]byte, 2*1024*1024) // 2MB
	for i := range data {
		data[i] = 'a'
	}
	result := ValidateSkillUpload(data, "big.py", 1) // 1MB limit
	if result.Passed {
		t.Error("expected oversized file to be rejected")
	}
}

// TestValidateSkillUpload_NullBytes rejects binary content.
func TestValidateSkillUpload_NullBytes(t *testing.T) {
	data := []byte("normal code\x00hidden binary")
	result := ValidateSkillUpload(data, "tricky.py", 1)
	if result.Passed {
		t.Error("expected file with null bytes to be rejected")
	}
}

// TestValidateSkillUpload_WithWarnings passes but records warnings.
func TestValidateSkillUpload_WithWarnings(t *testing.T) {
	code := []byte(`import os
os.system("ls")
`)
	result := ValidateSkillUpload(code, "risky.py", 1)
	if !result.Passed {
		t.Error("expected file with only warnings to pass")
	}
	if len(result.Findings) == 0 {
		t.Error("expected warnings to be recorded")
	}
}

// TestValidateSkillUpload_WithCritical fails with critical findings.
func TestValidateSkillUpload_WithCritical(t *testing.T) {
	code := []byte(`import pickle
data = pickle.load(open("data.pkl", "rb"))
`)
	result := ValidateSkillUpload(code, "dangerous.py", 1)
	if result.Passed {
		t.Error("expected file with critical findings to fail")
	}
}

// TestDetermineSecurityStatus_Clean returns clean for no findings.
func TestDetermineSecurityStatus_Clean(t *testing.T) {
	report := &SecurityReport{
		StaticAnalysis: []Finding{},
	}
	status := DetermineSecurityStatus(report)
	if status != SecurityClean {
		t.Errorf("expected SecurityClean, got %s", status)
	}
}

// TestDetermineSecurityStatus_Warning returns warning for warning-level findings.
func TestDetermineSecurityStatus_Warning(t *testing.T) {
	report := &SecurityReport{
		StaticAnalysis: []Finding{
			{Severity: "warning", Category: "exec", Message: "eval() usage"},
		},
	}
	status := DetermineSecurityStatus(report)
	if status != SecurityWarning {
		t.Errorf("expected SecurityWarning, got %s", status)
	}
}

// TestDetermineSecurityStatus_Dangerous returns dangerous for critical findings.
func TestDetermineSecurityStatus_Dangerous(t *testing.T) {
	report := &SecurityReport{
		StaticAnalysis: []Finding{
			{Severity: "critical", Category: "injection", Message: "pickle.load()"},
		},
	}
	status := DetermineSecurityStatus(report)
	if status != SecurityDangerous {
		t.Errorf("expected SecurityDangerous, got %s", status)
	}
}

// TestDetermineSecurityStatus_VTFlagged returns dangerous when VT flags.
func TestDetermineSecurityStatus_VTFlagged(t *testing.T) {
	report := &SecurityReport{
		StaticAnalysis:  []Finding{},
		VirusTotalScore: 0.5,
	}
	status := DetermineSecurityStatus(report)
	if status != SecurityDangerous {
		t.Errorf("expected SecurityDangerous for VT-flagged, got %s", status)
	}
}

// TestDetermineSecurityStatus_GuardianBlock returns dangerous when guardian blocks.
func TestDetermineSecurityStatus_GuardianBlock(t *testing.T) {
	report := &SecurityReport{
		StaticAnalysis:  []Finding{},
		GuardianVerdict: "block",
	}
	status := DetermineSecurityStatus(report)
	if status != SecurityDangerous {
		t.Errorf("expected SecurityDangerous for guardian block, got %s", status)
	}
}

// TestDetermineSecurityStatus_GuardianSuspicious returns warning for suspicious.
func TestDetermineSecurityStatus_GuardianSuspicious(t *testing.T) {
	report := &SecurityReport{
		StaticAnalysis:  []Finding{},
		GuardianVerdict: "suspicious",
	}
	status := DetermineSecurityStatus(report)
	if status != SecurityWarning {
		t.Errorf("expected SecurityWarning for guardian suspicious, got %s", status)
	}
}

// TestDetermineSecurityStatus_Nil returns pending for nil report.
func TestDetermineSecurityStatus_Nil(t *testing.T) {
	status := DetermineSecurityStatus(nil)
	if status != SecurityPending {
		t.Errorf("expected SecurityPending for nil report, got %s", status)
	}
}
