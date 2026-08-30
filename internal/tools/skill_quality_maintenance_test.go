package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPythonSkillQualityProvenanceUsageAndTombstone(t *testing.T) {
	mgr, skillsDir := setupTestSkillManager(t)
	agentEntry, err := mgr.CreateSkillEntry("agent_quality", "agent", "def run():\n    return 'ok'\n", SkillTypeAgent, "agent", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	userEntry, err := mgr.CreateSkillEntry("user_quality", "user", "def run():\n    return 'ok'\n", SkillTypeUser, "user", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	unknownManifest, _ := json.Marshal(SkillManifest{Name: "disk_unknown", Description: "unknown", Executable: "disk_unknown.py"})
	if err := os.WriteFile(filepath.Join(skillsDir, "disk_unknown.json"), unknownManifest, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "disk_unknown.py"), []byte("def run():\n    return 'unknown'\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	templateManifest, _ := json.Marshal(SkillManifest{Name: "template_user", Description: "UI template", Executable: "template_user.py"})
	if err := os.WriteFile(filepath.Join(skillsDir, "template_user.json"), templateManifest, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "template_user.py"), []byte("def run():\n    return 'user'\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := mgr.SyncFromDiskWithOrigins(map[string]SkillOrigin{"template_user": OriginUser}); err != nil {
		t.Fatal(err)
	}

	agentEntry, _ = mgr.GetSkill(agentEntry.ID)
	userEntry, _ = mgr.GetSkill(userEntry.ID)
	unknownEntry, err := mgr.GetSkillByName("disk_unknown")
	if err != nil {
		t.Fatal(err)
	}
	if agentEntry.Origin != OriginAgent || userEntry.Origin != OriginUser || unknownEntry.Origin != OriginLegacyUnknown {
		t.Fatalf("origins = agent:%s user:%s unknown:%s", agentEntry.Origin, userEntry.Origin, unknownEntry.Origin)
	}
	templateEntry, err := mgr.GetSkillByName("template_user")
	if err != nil || templateEntry.Origin != OriginUser || templateEntry.Type != SkillTypeUser {
		t.Fatalf("template entry=%+v err=%v", templateEntry, err)
	}
	if err := mgr.RecordSkillUsage(agentEntry.Name, true); err != nil {
		t.Fatal(err)
	}
	if err := mgr.RecordSkillUsage(agentEntry.Name, false); err != nil {
		t.Fatal(err)
	}
	agentEntry, _ = mgr.GetSkill(agentEntry.ID)
	if agentEntry.Usage.Attempts != 2 || agentEntry.Usage.Successes != 1 || agentEntry.Usage.Failures != 1 || agentEntry.Usage.LastUsedAt == nil {
		t.Fatalf("usage = %+v", agentEntry.Usage)
	}
	candidates, err := mgr.ListPythonQualityCandidates(20)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].ID != agentEntry.ID {
		t.Fatalf("candidates = %+v, want only proven agent skill", candidates)
	}
	if err := mgr.DeletePythonSkillForMaintenance(candidates[0], 0.99, "placeholder implementation"); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.GetSkill(agentEntry.ID); err == nil {
		t.Fatal("deleted agent skill remains in registry")
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "agent_quality.py")); !os.IsNotExist(err) {
		t.Fatalf("deleted source still exists: %v", err)
	}
	if _, err := mgr.GetSkill(userEntry.ID); err != nil {
		t.Fatalf("user skill was modified: %v", err)
	}
	var versions, tombstones, ordinaryAudits int
	if err := mgr.db.QueryRow("SELECT COUNT(*) FROM skill_versions WHERE skill_id = ?", agentEntry.ID).Scan(&versions); err != nil {
		t.Fatal(err)
	}
	if err := mgr.db.QueryRow("SELECT COUNT(*) FROM skill_quality_maintenance_log WHERE skill_id = ? AND decision = 'deleted'", agentEntry.ID).Scan(&tombstones); err != nil {
		t.Fatal(err)
	}
	if err := mgr.db.QueryRow("SELECT COUNT(*) FROM skill_audit_log WHERE skill_id = ?", agentEntry.ID).Scan(&ordinaryAudits); err != nil {
		t.Fatal(err)
	}
	if versions != 0 || ordinaryAudits != 0 || tombstones != 1 {
		t.Fatalf("versions=%d ordinary_audits=%d tombstones=%d", versions, ordinaryAudits, tombstones)
	}
}

func TestSkillQualityMigrationsAreIdempotentAndConservative(t *testing.T) {
	mgr, _ := setupTestSkillManager(t)
	if err := migrateSkillQualityColumns(mgr.db, "skills_registry"); err != nil {
		t.Fatalf("second quality migration failed: %v", err)
	}
	_, err := mgr.db.Exec(`INSERT INTO skills_registry (id, name, type, description, executable, created_by, origin) VALUES
		('legacy-no-proof', 'legacy_no_proof', 'agent', '', 'legacy_no_proof.py', 'agent', 'legacy_unknown'),
		('legacy-proof', 'legacy_proof', 'agent', '', 'legacy_proof.py', 'agent', 'legacy_unknown')`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.db.Exec(`INSERT INTO skill_versions (skill_id, version_num, code_hash, code, created_by) VALUES ('legacy-proof', 1, 'hash', 'pass', 'agent')`); err != nil {
		t.Fatal(err)
	}
	if err := backfillPythonSkillOrigins(mgr.db); err != nil {
		t.Fatal(err)
	}
	var noProof, proof string
	if err := mgr.db.QueryRow("SELECT origin FROM skills_registry WHERE id = 'legacy-no-proof'").Scan(&noProof); err != nil {
		t.Fatal(err)
	}
	if err := mgr.db.QueryRow("SELECT origin FROM skills_registry WHERE id = 'legacy-proof'").Scan(&proof); err != nil {
		t.Fatal(err)
	}
	if noProof != string(OriginLegacyUnknown) || proof != string(OriginAgent) {
		t.Fatalf("origins without proof=%q with proof=%q", noProof, proof)
	}
}

func TestVirusTotalMaintenanceResultRequiresCleanValidResponse(t *testing.T) {
	clean, err := virusTotalMaintenanceResultClean(`{"status":"success","stats":{"malicious":0,"suspicious":0}}`)
	if err != nil || !clean {
		t.Fatalf("clean response: clean=%t err=%v", clean, err)
	}
	clean, err = virusTotalMaintenanceResultClean(`{"status":"success","stats":{"malicious":1}}`)
	if err != nil || clean {
		t.Fatalf("flagged response: clean=%t err=%v", clean, err)
	}
	if _, err := virusTotalMaintenanceResultClean(`{"status":"error","message":"unavailable"}`); err == nil {
		t.Fatal("VirusTotal error response was accepted")
	}
	if _, err := virusTotalMaintenanceResultClean(`not-json`); err == nil {
		t.Fatal("invalid VirusTotal response was accepted")
	}
}

func TestPythonSkillQualityRevisionRollsBackOnValidationFailure(t *testing.T) {
	mgr, _ := setupTestSkillManager(t)
	entry, err := mgr.CreateSkillEntry("agent_revision", "agent", "def run():\n    return 'original'\n", SkillTypeAgent, "agent", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := mgr.ListPythonQualityCandidates(10)
	if err != nil || len(candidates) != 1 {
		t.Fatalf("candidates=%+v err=%v", candidates, err)
	}
	err = mgr.ApplyPythonSkillQualityRevision(context.Background(), candidates[0], "def run(:\n    pass\n", 0.99, "bad revision", nil, false, false, "", SkillSpectorConfig{})
	if err == nil {
		t.Fatal("invalid syntax was accepted")
	}
	code, readErr := mgr.GetSkillCode(entry.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(code, "original") {
		t.Fatalf("original code was not preserved: %q", code)
	}
	refreshed, _ := mgr.GetSkill(entry.ID)
	if refreshed.FileHash != entry.FileHash {
		t.Fatalf("hash changed after rejected revision: %s != %s", refreshed.FileHash, entry.FileHash)
	}
}

func TestPythonSkillQualityRevisionRefusesDiskDrift(t *testing.T) {
	mgr, skillsDir := setupTestSkillManager(t)
	entry, err := mgr.CreateSkillEntry("agent_disk_drift", "agent", "def run():\n    return 'reviewed'\n", SkillTypeAgent, "agent", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := mgr.ListPythonQualityCandidates(10)
	if err != nil || len(candidates) != 1 {
		t.Fatalf("candidates=%+v err=%v", candidates, err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, entry.Executable), []byte("def run():\n    return 'external edit'\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := mgr.ApplyPythonSkillQualityRevision(context.Background(), candidates[0], "def run():\n    return 'model edit'\n", 0.99, "improve", nil, false, false, "", SkillSpectorConfig{}); err == nil {
		t.Fatal("maintenance overwrote a skill that drifted on disk")
	}
	code, err := mgr.GetSkillCode(entry.ID)
	if err != nil || !strings.Contains(code, "external edit") {
		t.Fatalf("external edit was not preserved: code=%q err=%v", code, err)
	}
}

func TestPythonSkillQualityRevisionCommitsCleanVersion(t *testing.T) {
	if findSystemPython() == "" {
		t.Skip("system Python unavailable")
	}
	mgr, _ := setupTestSkillManager(t)
	entry, err := mgr.CreateSkillEntry("agent_clean_revision", "agent", "def run():\n    return 'original'\n", SkillTypeAgent, "agent", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := mgr.ListPythonQualityCandidates(10)
	if err != nil || len(candidates) != 1 {
		t.Fatalf("candidates=%+v err=%v", candidates, err)
	}
	if err := mgr.ApplyPythonSkillQualityRevision(context.Background(), candidates[0], "def run():\n    return 'improved'\n", 0.97, "improve useful behavior", nil, false, false, "", SkillSpectorConfig{}); err != nil {
		t.Fatal(err)
	}
	code, err := mgr.GetSkillCode(entry.ID)
	if err != nil || !strings.Contains(code, "improved") {
		t.Fatalf("code=%q err=%v", code, err)
	}
	updated, err := mgr.GetSkill(entry.ID)
	if err != nil || updated.Origin != OriginAgent || updated.LastQualityVerdict != "improved" || updated.LastQualityConfidence != 0.97 {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
	var maintenanceVersions int
	if err := mgr.db.QueryRow("SELECT COUNT(*) FROM skill_versions WHERE skill_id = ? AND created_by = 'maintenance'", entry.ID).Scan(&maintenanceVersions); err != nil {
		t.Fatal(err)
	}
	if maintenanceVersions != 1 {
		t.Fatalf("maintenance versions=%d", maintenanceVersions)
	}
}

func TestPythonSkillQualityDeletionWaitsForActiveExecution(t *testing.T) {
	mgr, _ := setupTestSkillManager(t)
	_, err := mgr.CreateSkillEntry("agent_execution_lease", "agent", "def run():\n    return 'ok'\n", SkillTypeAgent, "agent", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := mgr.ListPythonQualityCandidates(10)
	if err != nil || len(candidates) != 1 {
		t.Fatalf("candidates=%+v err=%v", candidates, err)
	}
	release := mgr.AcquireSkillExecutionLease()
	done := make(chan error, 1)
	go func() { done <- mgr.DeletePythonSkillForMaintenance(candidates[0], 0.99, "placeholder") }()
	select {
	case err := <-done:
		t.Fatalf("deletion completed while execution lease was active: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	release()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("deletion did not resume after execution lease release")
	}
}

func TestAgentSkillQualityProvenanceUsageAndTombstone(t *testing.T) {
	mgr, _ := setupAgentSkillManagerOnDisk(t)
	agentEntry, err := mgr.CreateAgentSkill(context.Background(), "agent-maintained", "Agent maintained test skill. Use for tests.", "# Agent maintained\n\nDo useful work.", "agent", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	userEntry, err := mgr.CreateAgentSkill(context.Background(), "user-maintained", "User maintained test skill. Use for tests.", "# User maintained\n\nDo useful work.", "user", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if agentEntry.Origin != OriginAgent || userEntry.Origin != OriginUser {
		t.Fatalf("origins = agent:%s user:%s", agentEntry.Origin, userEntry.Origin)
	}
	if err := mgr.RecordAgentSkillUsage(agentEntry.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.db.Exec(`INSERT INTO agent_skill_versions (skill_id, version_num, package_hash, package_snapshot, created_by) VALUES (?, 1, ?, 'snapshot', 'maintenance')`, agentEntry.ID, agentEntry.PackageHash); err != nil {
		t.Fatal(err)
	}
	candidates, err := mgr.ListAgentSkillQualityCandidates(10)
	if err != nil || len(candidates) != 1 || candidates[0].ID != agentEntry.ID {
		t.Fatalf("candidates=%+v err=%v", candidates, err)
	}
	if err := mgr.DeleteAgentSkillForMaintenance(candidates[0], 0.99, "test-only placeholder"); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.GetAgentSkill(agentEntry.ID); err == nil {
		t.Fatal("deleted Agent Skill remains in registry")
	}
	if _, err := mgr.GetAgentSkill(userEntry.ID); err != nil {
		t.Fatalf("user Agent Skill was modified: %v", err)
	}
	var tombstone, versions, ordinaryAudits int
	if err := mgr.db.QueryRow("SELECT COUNT(*) FROM skill_quality_maintenance_log WHERE skill_id = ? AND decision = 'deleted'", agentEntry.ID).Scan(&tombstone); err != nil {
		t.Fatal(err)
	}
	if tombstone != 1 {
		t.Fatalf("tombstones=%d", tombstone)
	}
	if err := mgr.db.QueryRow("SELECT COUNT(*) FROM agent_skill_versions WHERE skill_id = ?", agentEntry.ID).Scan(&versions); err != nil {
		t.Fatal(err)
	}
	if versions != 0 {
		t.Fatalf("Agent Skill versions remain after deletion: %d", versions)
	}
	if err := mgr.db.QueryRow("SELECT COUNT(*) FROM agent_skill_audit_log WHERE skill_id = ?", agentEntry.ID).Scan(&ordinaryAudits); err != nil {
		t.Fatal(err)
	}
	if ordinaryAudits != 0 {
		t.Fatalf("Agent Skill ordinary audits remain after deletion: %d", ordinaryAudits)
	}
}

func TestAgentSkillQualityRevisionPreservesContract(t *testing.T) {
	mgr, _ := setupAgentSkillManagerOnDisk(t)
	entry, err := mgr.CreateAgentSkill(context.Background(), "agent-contract", "Agent contract test skill. Use for tests.", "# Agent contract\n\nOriginal instructions.", "agent", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := mgr.ListAgentSkillQualityCandidates(10)
	if err != nil || len(candidates) != 1 {
		t.Fatalf("candidates=%+v err=%v", candidates, err)
	}
	raw, err := os.ReadFile(filepath.Join(entry.Directory, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	revised := strings.Replace(string(raw), "Original instructions.", "Improved, precise instructions.", 1)
	if err := mgr.ApplyAgentSkillQualityRevision(context.Background(), candidates[0], map[string]string{"SKILL.md": revised}, 0.97, "improve instructions", nil, false, SkillSpectorConfig{}); err != nil {
		t.Fatal(err)
	}
	updated, err := mgr.GetAgentSkill(entry.ID)
	if err != nil || updated.PackageHash == entry.PackageHash || updated.Origin != OriginAgent || updated.LastQualityVerdict != "improved" {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
	contractExpansion := strings.Replace(revised, "---\n# Agent contract", "allowed-tools: Shell\n---\n# Agent contract", 1)
	refreshedCandidates, err := mgr.ListAgentSkillQualityCandidates(10)
	if err != nil || len(refreshedCandidates) != 0 {
		// A clean, unchanged skill is intentionally not due again for 30 days.
		if err != nil {
			t.Fatal(err)
		}
	}
	manualCandidate := candidates[0]
	manualCandidate.ContentHash = updated.PackageHash
	if err := mgr.ApplyAgentSkillQualityRevision(context.Background(), manualCandidate, map[string]string{"SKILL.md": contractExpansion}, 0.99, "expand tools", nil, false, SkillSpectorConfig{}); err == nil {
		t.Fatal("allowed-tools expansion was accepted")
	}
	stable, _ := mgr.GetAgentSkill(entry.ID)
	if stable.PackageHash != updated.PackageHash {
		t.Fatal("rejected contract expansion changed package")
	}
}
