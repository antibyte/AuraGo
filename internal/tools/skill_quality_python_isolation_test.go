package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPythonQualityValidationIgnoresAgentVenvCWDAndPythonPath(t *testing.T) {
	mgr, skillsDir := setupTestSkillManager(t)
	jail := t.TempDir()
	mgr = NewSkillManager(mgr.db, skillsDir, mgr.logger, jail)
	if python := trustedSupervisorBootstrapPython(jail); python == "" {
		t.Fatal("trusted system Python is required for this security regression")
	}
	entry, err := mgr.CreateSkillEntry("isolated_quality", "agent", "def run():\n    return 'original'\n", SkillTypeAgent, "agent", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(jail)
	venvPython := GetPythonBin("")
	if err := os.MkdirAll(filepath.Dir(venvPython), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(venvPython, []byte("untrusted agent interpreter"), 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(jail, "host-hook-ran")
	hook := fmt.Sprintf("open(%q, 'w').write('untrusted')\nraise RuntimeError('untrusted import executed')\n", marker)
	pythonPath := filepath.Join(jail, "pythonpath")
	if err := os.MkdirAll(pythonPath, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{jail, pythonPath} {
		for _, module := range []string{"sitecustomize.py", "py_compile.py", "ast.py", "json.py"} {
			if err := os.WriteFile(filepath.Join(dir, module), []byte(hook), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Setenv("PYTHONPATH", pythonPath)
	t.Setenv("AURAGO_MASTER_KEY", "quality-check-fixture-secret")
	ctx := context.Background()
	code := "raise RuntimeError('source must never execute during validation')\n\ndef run():\n    return 'improved'\n"
	if _, err := mgr.validatePythonQualityRevision(ctx, entry, code, nil, false, false, "", SkillSpectorConfig{}); err != nil {
		t.Fatalf("trusted syntax/contract validation inherited an agent interpreter or import path: %v", err)
	}
	// Exercise the shared contract helper directly as well: its current callers
	// must not make import isolation depend on an earlier syntax check.
	python := trustedSupervisorBootstrapPython(jail)
	contract, err := extractPythonCallableContract(ctx, python, filepath.Join(skillsDir, entry.Executable))
	if err != nil || !strings.Contains(contract, `"name":"run"`) {
		t.Fatalf("isolated callable contract failed: %q %v", contract, err)
	}
	probe := supervisorPythonCommand(ctx, python, "-c", "import os,sys; assert sys.flags.isolated; assert 'AURAGO_MASTER_KEY' not in os.environ; assert os.environ.get('PYTHONPATH') not in sys.path")
	if output, err := probe.CombinedOutput(); err != nil {
		t.Fatalf("trusted checks retained ambient secrets or import configuration: %v %s", err, output)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("agent-owned startup/import hook ran on the host: %v", err)
	}
	if original, err := mgr.GetSkillCode(entry.ID); err != nil || original != "def run():\n    return 'original'\n" {
		t.Fatalf("validation changed the installed skill: %q %v", original, err)
	}
}

func TestPythonQualityValidationRejectsOnlyAgentOwnedPython(t *testing.T) {
	mgr, skillsDir := setupTestSkillManager(t)
	jail := t.TempDir()
	mgr = NewSkillManager(mgr.db, skillsDir, mgr.logger, jail)
	entry, err := mgr.CreateSkillEntry("blocked_quality", "agent", "def run():\n    return 'original'\n", SkillTypeAgent, "agent", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(jail, "venv", "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"python", "python3"} {
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		if err := os.WriteFile(filepath.Join(bin, name), []byte("untrusted interpreter must never start"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	if _, err := mgr.validatePythonQualityRevision(context.Background(), entry, "def run():\n    return 'improved'\n", nil, false, false, "", SkillSpectorConfig{}); err == nil || !strings.Contains(err.Error(), "trusted system Python is unavailable") {
		t.Fatalf("agent-owned Python was accepted or executed: %v", err)
	}
}
