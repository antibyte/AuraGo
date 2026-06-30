# Homepage Registry Hard Identity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent new homepage registry entries and successful deploys from being created without a durable local project identity and deploy target.

**Architecture:** Add one shared homepage identity validator in `internal/tools`, enforce it at registry write boundaries, and introduce a strict deployment recording path that separates fatal ledger failures from non-fatal artifact warnings. The agent dispatch layer will convert fatal ledger failures into JSON error tool outputs so the agent cannot later perform cleanup from incomplete registry data.

**Tech Stack:** Go 1.26, standard library, modernc SQLite tests, existing AuraGo homepage tool and dispatch packages.

---

## Scope Check

This plan touches one cohesive subsystem: managed homepage registry identity and deploy target recording. It does not migrate existing rows, does not redesign the UI, and does not change remote provider APIs.

## File Structure

- Modify `internal/tools/homepage_validation.go`: add exported `NormalizeHomepageProjectIdentity` helper beside existing path validation.
- Modify `internal/tools/homepage_registry.go`: enforce `ProjectDir` in `RegisterProject` and `DispatchHomepageRegistry`.
- Modify `internal/tools/homepage_ledger.go`: use strict identity validation in `EnsureHomepageProjectForDir`, require deployment target location, and add strict deploy-result recording.
- Modify `internal/agent/agent_dispatch_services.go`: convert strict deployment ledger failures into error tool outputs for deploy/publish operations.
- Modify `internal/agent/native_tools_integrations.go`: mark `project_dir` required for `homepage_registry` register at tool-schema level by tightening the tool description and adding runtime guard in dispatch; keep schema-wide `required` unchanged because one schema covers many operations.
- Modify `internal/tools/homepage_registry_test.go`: update fixtures and add hard identity tests.
- Modify `internal/tools/homepage_ledger_test.go`: add strict deploy target tests and update direct deployment fixtures that currently have no URL/path.
- Modify `internal/agent/dispatch_homepage_test.go`: add dispatch-level hard failure tests for missing `project_dir` and failed deployment target recording.
- Modify `prompts/tools_manuals/homepage_registry.md`: document required `project_dir`.
- Modify `prompts/tools_manuals/homepage.md`: clarify that mutating/deploying project operations must carry `project_dir`.

## Pre-Work

- [ ] **Step 1: Refresh code intelligence and check impact before code edits**

Run:

```powershell
node .gitnexus/run.cjs analyze
npx gitnexus impact RegisterProject --repo 'C:\Users\Andi\Documents\repo\AuraGo'
npx gitnexus impact EnsureHomepageProjectForDir --repo 'C:\Users\Andi\Documents\repo\AuraGo'
npx gitnexus impact RecordHomepageDeploymentFromResult --repo 'C:\Users\Andi\Documents\repo\AuraGo'
```

Expected: GitNexus reports the current repo index and blast radius. If any impact result is HIGH or CRITICAL, report it before editing.

---

### Task 1: Project Identity Validator

**Files:**
- Modify: `internal/tools/homepage_validation.go`
- Test: `internal/tools/homepage_test.go`

- [ ] **Step 1: Write failing validator tests**

Add these tests to `internal/tools/homepage_test.go` near the existing `sanitizeProjectDir` tests:

```go
func TestNormalizeHomepageProjectIdentityRequiresProjectDir(t *testing.T) {
	_, err := NormalizeHomepageProjectIdentity("", false)
	if err == nil || !strings.Contains(err.Error(), "project_dir is required") {
		t.Fatalf("expected project_dir required error, got %v", err)
	}
}

func TestNormalizeHomepageProjectIdentityRejectsAmbiguousRoot(t *testing.T) {
	_, err := NormalizeHomepageProjectIdentity(".", false)
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguous root error, got %v", err)
	}
}

func TestNormalizeHomepageProjectIdentityNormalizesSafeRelativePath(t *testing.T) {
	got, err := NormalizeHomepageProjectIdentity("sites/site-a/", false)
	if err != nil {
		t.Fatalf("NormalizeHomepageProjectIdentity failed: %v", err)
	}
	if got != "sites/site-a" {
		t.Fatalf("normalized project_dir = %q, want sites/site-a", got)
	}
}

func TestNormalizeHomepageProjectIdentityRejectsTraversal(t *testing.T) {
	_, err := NormalizeHomepageProjectIdentity("sites/../site-a", false)
	if err == nil || !strings.Contains(err.Error(), "path traversal") {
		t.Fatalf("expected path traversal error, got %v", err)
	}
}

func TestNormalizeHomepageProjectIdentityRejectsAbsolutePath(t *testing.T) {
	_, err := NormalizeHomepageProjectIdentity("/workspace/site-a", false)
	if err == nil || !strings.Contains(err.Error(), "relative to the homepage workspace") {
		t.Fatalf("expected absolute path error, got %v", err)
	}
}
```

- [ ] **Step 2: Run validator tests and confirm failure**

Run:

```powershell
go test ./internal/tools/... -run "TestNormalizeHomepageProjectIdentity" -count=1
```

Expected: FAIL because `NormalizeHomepageProjectIdentity` is undefined.

- [ ] **Step 3: Add the validator**

Add this function below `sanitizeProjectDir` in `internal/tools/homepage_validation.go`:

```go
// NormalizeHomepageProjectIdentity returns the canonical registry identity for a
// homepage project. It accepts only workspace-relative project directories.
func NormalizeHomepageProjectIdentity(projectDir string, allowRoot bool) (string, error) {
	projectDir = strings.TrimSpace(filepath.ToSlash(projectDir))
	if projectDir == "" {
		return "", fmt.Errorf("project_dir is required to register a homepage project")
	}
	if filepath.IsAbs(projectDir) {
		return "", fmt.Errorf("project_dir must be relative to the homepage workspace")
	}
	projectDir = strings.Trim(projectDir, "/")
	if err := sanitizeProjectDir(projectDir); err != nil {
		return "", err
	}
	projectDir = strings.Trim(filepath.ToSlash(filepath.Clean(filepath.FromSlash(projectDir))), "/")
	if projectDir == "" || projectDir == "." {
		if allowRoot {
			return ".", nil
		}
		return "", fmt.Errorf(`project_dir "." is ambiguous for new homepage projects`)
	}
	return projectDir, nil
}
```

Add `path/filepath` to the imports in `internal/tools/homepage_validation.go` if it is not already present:

```go
import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)
```

- [ ] **Step 4: Run validator tests and confirm pass**

Run:

```powershell
go test ./internal/tools/... -run "TestNormalizeHomepageProjectIdentity" -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit validator**

Run:

```powershell
git add internal/tools/homepage_validation.go internal/tools/homepage_test.go
git commit -m "Enforce homepage project identity validation"
```

---

### Task 2: Registry Register Hardening

**Files:**
- Modify: `internal/tools/homepage_registry.go`
- Modify: `internal/tools/homepage_registry_test.go`

- [ ] **Step 1: Write failing registry tests**

Add these tests after `TestRegisterAndGetProject` in `internal/tools/homepage_registry_test.go`:

```go
func TestRegisterProjectRequiresProjectDir(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	_, _, err = RegisterProject(db, HomepageProject{Name: "MissingDir"})
	if err == nil || !strings.Contains(err.Error(), "project_dir is required") {
		t.Fatalf("expected project_dir required error, got %v", err)
	}
}

func TestDispatchHomepageRegistryRegisterRequiresProjectDir(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	got := DispatchHomepageRegistry(db, "register", "", "MissingDir", "", "html", "", "", "", "", "", "", nil, 0, "", 10, 0)
	if !strings.Contains(got, `"status":"error"`) || !strings.Contains(got, "project_dir is required") {
		t.Fatalf("expected missing project_dir error, got %s", got)
	}
}
```

Update existing registry test fixtures that call `RegisterProject` without `ProjectDir`:

```go
RegisterProject(db, HomepageProject{Name: "SiteA", ProjectDir: "site-a", Framework: "vue"})
RegisterProject(db, HomepageProject{Name: "Portfolio", ProjectDir: "portfolio", Description: "Personal site", Framework: "astro"})
RegisterProject(db, HomepageProject{Name: "Blog", ProjectDir: "blog", Description: "Tech blog", Framework: "hugo"})
id, _, _ := RegisterProject(db, HomepageProject{Name: "EditTest", ProjectDir: "edit-test", Framework: "next"})
id, _, _ := RegisterProject(db, HomepageProject{Name: "ProblemTest", ProjectDir: "problem-test", Framework: "gatsby"})
RegisterProject(db, HomepageProject{Name: "A", ProjectDir: "site-a", Status: "active"})
RegisterProject(db, HomepageProject{Name: "B", ProjectDir: "site-b", Status: "archived"})
RegisterProject(db, HomepageProject{Name: "C", ProjectDir: "site-c", Status: "active"})
```

- [ ] **Step 2: Run registry tests and confirm failure**

Run:

```powershell
go test ./internal/tools/... -run "TestRegister|TestDispatchHomepageRegistryRegister" -count=1
```

Expected: FAIL until `RegisterProject` and dispatch enforce `project_dir`.

- [ ] **Step 3: Enforce and normalize `ProjectDir` in `RegisterProject`**

In `internal/tools/homepage_registry.go`, insert this block after the nil DB check in `RegisterProject`:

```go
	projectDir, err := NormalizeHomepageProjectIdentity(p.ProjectDir, false)
	if err != nil {
		return 0, false, err
	}
	p.ProjectDir = projectDir
```

The start of the function should read:

```go
func RegisterProject(db *sql.DB, p HomepageProject) (int64, bool, error) {
	if db == nil {
		return 0, false, fmt.Errorf("homepage registry DB not initialized")
	}
	projectDir, err := NormalizeHomepageProjectIdentity(p.ProjectDir, false)
	if err != nil {
		return 0, false, err
	}
	p.ProjectDir = projectDir

	// Dedup by name
```

- [ ] **Step 4: Add explicit dispatch guard**

In `DispatchHomepageRegistry`, replace the current `register` case guard with:

```go
	case "register":
		if name == "" {
			return `{"status":"error","message":"'name' is required to register a project."}`
		}
		if _, err := NormalizeHomepageProjectIdentity(projectDir, false); err != nil {
			return fmt.Sprintf(`{"status":"error","message":"%s"}`, strings.ReplaceAll(err.Error(), `"`, `'`))
		}
```

Keep the existing `HomepageProject` construction after this guard. `RegisterProject` will normalize the stored value.

- [ ] **Step 5: Run registry tests and confirm pass**

Run:

```powershell
go test ./internal/tools/... -run "TestRegister|TestDispatchHomepageRegistryRegister|TestGetProject|TestSearchProjects|TestLog" -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit registry hardening**

Run:

```powershell
git add internal/tools/homepage_registry.go internal/tools/homepage_registry_test.go
git commit -m "Require project_dir for homepage registry entries"
```

---

### Task 3: Ledger Canonicalization and Deploy Target Requirements

**Files:**
- Modify: `internal/tools/homepage_ledger.go`
- Modify: `internal/tools/homepage_ledger_test.go`

- [ ] **Step 1: Write failing ledger tests**

Add these tests after `TestEnsureHomepageProjectForDirCanonicalizesAndCreatesState` in `internal/tools/homepage_ledger_test.go`:

```go
func TestEnsureHomepageProjectForDirRejectsAmbiguousRoot(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	_, err := EnsureHomepageProjectForDir(db, HomepageConfig{WorkspacePath: t.TempDir()}, "", "Root", "html")
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguous root error, got %v", err)
	}
}

func TestRecordHomepageDeploymentRequiresTargetLocation(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	workspace := t.TempDir()
	proj, err := EnsureHomepageProjectForDir(db, HomepageConfig{WorkspacePath: workspace}, "site-a", "site-a", "html")
	if err != nil {
		t.Fatalf("ensure project: %v", err)
	}

	err = RecordHomepageDeployment(db, HomepageDeploymentRecord{
		ProjectID: proj.ID,
		Provider:  "local",
		Status:    "ok",
	})
	if err == nil || !strings.Contains(err.Error(), "deploy URL or remote_path") {
		t.Fatalf("expected target location error, got %v", err)
	}
}
```

Update direct deployment fixtures that intentionally use local provider without URL:

```go
RemotePath: filepath.Join(workspace, projectDir),
```

For `TestReconcileHomepageProjectDetectsUndeployedLatestRevision`, the initial deploy record should become:

```go
	if err := RecordHomepageDeployment(db, HomepageDeploymentRecord{
		ProjectID:  proj.ID,
		RevisionID: initial.RevisionID,
		Provider:   "local",
		RemotePath: filepath.Join(workspace, projectDir),
		BuildDir:   ".",
		Status:     "ok",
	}); err != nil {
		t.Fatalf("record initial deploy: %v", err)
	}
```

- [ ] **Step 2: Run ledger tests and confirm failure**

Run:

```powershell
go test ./internal/tools/... -run "TestEnsureHomepageProjectForDir|TestRecordHomepageDeployment" -count=1
```

Expected: FAIL until root identity and target location are enforced.

- [ ] **Step 3: Enforce project identity in `EnsureHomepageProjectForDir`**

In `internal/tools/homepage_ledger.go`, replace:

```go
	if canonical == "" {
		canonical = "."
	}
```

with:

```go
	if canonical, err = NormalizeHomepageProjectIdentity(canonical, false); err != nil {
		return nil, err
	}
```

Keep `canonicalHomepageProjectDir` as the function that converts absolute workspace paths to relative paths. `NormalizeHomepageProjectIdentity` should run after that conversion.

- [ ] **Step 4: Enforce target location in `RecordHomepageDeployment`**

In `RecordHomepageDeployment`, after provider normalization and before defaulting status, insert:

```go
	rec.URL = strings.TrimSpace(rec.URL)
	rec.RemotePath = strings.TrimSpace(rec.RemotePath)
	if rec.URL == "" && rec.RemotePath == "" {
		return fmt.Errorf("deployment target could not be recorded; deploy URL or remote_path is required")
	}
```

The block should sit after:

```go
	rec.Provider = strings.ToLower(strings.TrimSpace(rec.Provider))
	if rec.Provider == "" {
		return fmt.Errorf("provider is required")
	}
```

- [ ] **Step 5: Run ledger tests and confirm pass**

Run:

```powershell
go test ./internal/tools/... -run "TestEnsureHomepageProjectForDir|TestRecordHomepageDeployment|TestReconcileHomepageProject" -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit ledger invariant**

Run:

```powershell
git add internal/tools/homepage_ledger.go internal/tools/homepage_ledger_test.go
git commit -m "Require homepage deployment target locations"
```

---

### Task 4: Strict Deployment Recording API

**Files:**
- Modify: `internal/tools/homepage_ledger.go`
- Modify: `internal/tools/homepage_ledger_test.go`

- [ ] **Step 1: Write failing strict recording tests**

Add these tests after `TestRecordHomepageDeploymentFromResultParsesProviderFields`:

```go
func TestRecordHomepageDeploymentFromResultStrictRejectsMissingTargetLocation(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	workspace := t.TempDir()
	cfg := HomepageConfig{WorkspacePath: workspace}
	if _, err := EnsureHomepageProjectForDir(db, cfg, "site-a", "site-a", "html"); err != nil {
		t.Fatalf("ensure project: %v", err)
	}

	warnings, err := RecordHomepageDeploymentFromResultStrict(cfg, db, "site-a", "netlify", ".", `{"status":"ok","id":"deploy-1","site_id":"site-1"}`, slog.Default())
	if err == nil || !strings.Contains(err.Error(), "deploy URL or remote_path") {
		t.Fatalf("expected strict target error, got warnings=%v err=%v", warnings, err)
	}
	var deployments int
	if countErr := db.QueryRow("SELECT COUNT(*) FROM homepage_deployments").Scan(&deployments); countErr != nil {
		t.Fatalf("count deployments: %v", countErr)
	}
	if deployments != 0 {
		t.Fatalf("deployments = %d, want 0", deployments)
	}
}

func TestRecordHomepageDeploymentFromResultStrictKeepsArtifactWarningsNonFatal(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	workspace := t.TempDir()
	cfg := HomepageConfig{WorkspacePath: workspace}
	if _, err := EnsureHomepageProjectForDir(db, cfg, "site-a", "site-a", "html"); err != nil {
		t.Fatalf("ensure project: %v", err)
	}

	warnings, err := RecordHomepageDeploymentFromResultStrict(cfg, db, "site-a", "local", "missing-build", `{"status":"ok","url":"http://localhost:8080","project_dir":"site-a","build_dir":"missing-build"}`, slog.Default())
	if err != nil {
		t.Fatalf("strict record should keep artifact manifest failure non-fatal, err=%v warnings=%v", err, warnings)
	}
	if len(warnings) == 0 {
		t.Fatal("expected artifact warning for missing build directory")
	}
	var deployments int
	if countErr := db.QueryRow("SELECT COUNT(*) FROM homepage_deployments").Scan(&deployments); countErr != nil {
		t.Fatalf("count deployments: %v", countErr)
	}
	if deployments != 1 {
		t.Fatalf("deployments = %d, want 1", deployments)
	}
}
```

Add `strings` to the imports in `internal/tools/homepage_ledger_test.go` if it is not already present.

- [ ] **Step 2: Run strict recording tests and confirm failure**

Run:

```powershell
go test ./internal/tools/... -run "TestRecordHomepageDeploymentFromResultStrict" -count=1
```

Expected: FAIL because `RecordHomepageDeploymentFromResultStrict` is undefined.

- [ ] **Step 3: Add strict recording result type and wrappers**

In `internal/tools/homepage_ledger.go`, add this type below `HomepageLedgerResult`:

```go
// HomepageDeploymentLedgerResult separates fatal deployment ledger failures
// from metadata warnings such as missing artifact manifests.
type HomepageDeploymentLedgerResult struct {
	Warnings []string
	Fatal    error
}
```

Replace `RecordHomepageDeploymentFromResult` with wrappers around a shared helper:

```go
func RecordHomepageDeploymentFromResult(cfg HomepageConfig, db *sql.DB, projectDir, provider, buildDir, rawResult string, logger *slog.Logger) []string {
	out := recordHomepageDeploymentFromResult(cfg, db, projectDir, provider, buildDir, rawResult, logger)
	warnings := append([]string{}, out.Warnings...)
	if out.Fatal != nil {
		warnings = append(warnings, out.Fatal.Error())
	}
	return warnings
}

func RecordHomepageDeploymentFromResultStrict(cfg HomepageConfig, db *sql.DB, projectDir, provider, buildDir, rawResult string, logger *slog.Logger) ([]string, error) {
	out := recordHomepageDeploymentFromResult(cfg, db, projectDir, provider, buildDir, rawResult, logger)
	return out.Warnings, out.Fatal
}

func recordHomepageDeploymentFromResult(cfg HomepageConfig, db *sql.DB, projectDir, provider, buildDir, rawResult string, logger *slog.Logger) HomepageDeploymentLedgerResult {
	var out HomepageDeploymentLedgerResult
	if db == nil {
		out.Fatal = fmt.Errorf("homepage registry DB not initialized")
		return out
	}
	parsed := map[string]interface{}{}
	if err := json.Unmarshal([]byte(rawResult), &parsed); err != nil {
		out.Fatal = fmt.Errorf("deployment result was not JSON: %v", err)
		return out
	}
	if status, _ := parsed["status"].(string); status == "error" {
		return out
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	if fallbackProjectDir := ledgerString(parsed, "fallback_project_dir"); fallbackProjectDir != "" {
		projectDir = fallbackProjectDir
	}
	if fallbackBuildDir := ledgerString(parsed, "fallback_build_dir"); fallbackBuildDir != "" {
		buildDir = fallbackBuildDir
	}
	if parsedProjectDir := ledgerString(parsed, "project_dir"); parsedProjectDir != "" && provider != "netlify" {
		projectDir = parsedProjectDir
	}
	proj, err := EnsureHomepageProjectForDir(db, cfg, projectDir, "", "")
	if err != nil {
		out.Fatal = err
		return out
	}
	if buildDir == "" {
		buildDir = ledgerString(parsed, "build_dir")
	}
	if buildDir == "" {
		buildDir = ledgerBuildDirFromDeployPath(proj.ProjectDir, ledgerString(parsed, "deploy_path"))
	}
	if buildDir == "" {
		buildDir = "."
	}
	latest, _ := GetLatestHomepageRevision(db, proj.ProjectDir)
	revisionID := int64(0)
	if latest != nil {
		revisionID = latest.ID
	}
	gitSHA := HomepageProjectGitSHA(cfg, proj.ProjectDir)
	artifactHash := ""
	if manifest, err := BuildHomepageArtifactManifest(cfg, proj.ID, proj.ProjectDir, revisionID, gitSHA, buildDir); err == nil {
		artifactHash = manifest.ArtifactHash
		if writeErr := WriteHomepageArtifactManifest(cfg, manifest); writeErr != nil {
			out.Warnings = append(out.Warnings, fmt.Sprintf("artifact manifest write failed: %v", writeErr))
		}
	} else {
		if logger != nil {
			logger.Warn("[Homepage] Artifact manifest failed", "project_dir", proj.ProjectDir, "build_dir", buildDir, "error", err)
		}
		out.Warnings = append(out.Warnings, fmt.Sprintf("artifact manifest failed: %v", err))
	}
	targetID := firstLedgerString(parsed, "site_id", "new_site_id", "deploy_site_id", "project_id")
	deployID := firstLedgerString(parsed, "deployment_id", "deploy_id", "id")
	url := firstLedgerString(parsed, "verified_url", "deployment_url", "deploy_url", "url", "served_url", "deploy_ssl_url", "deploy_deploy_url")
	remotePath := firstLedgerString(parsed, "path", "remote_path", "source_path")
	status := firstLedgerString(parsed, "status")
	if status == "" {
		status = "ok"
	}
	verification := map[string]interface{}{}
	if verified, ok := parsed["verified"].(bool); ok {
		verification["verified"] = verified
	}
	if verifiedURL := ledgerString(parsed, "verified_url"); verifiedURL != "" {
		verification["verified_url"] = verifiedURL
	}
	verificationJSON := ""
	if len(verification) > 0 {
		b, _ := json.Marshal(verification)
		verificationJSON = string(b)
	}
	if err := RecordHomepageDeployment(db, HomepageDeploymentRecord{
		ProjectID:        proj.ID,
		RevisionID:       revisionID,
		GitSHA:           gitSHA,
		Provider:         provider,
		ProviderTargetID: targetID,
		ProviderDeployID: deployID,
		URL:              url,
		RemotePath:       remotePath,
		BuildDir:         buildDir,
		ArtifactHash:     artifactHash,
		Status:           status,
		VerificationJSON: verificationJSON,
		Metadata:         parsed,
	}); err != nil {
		out.Fatal = err
	}
	return out
}
```

- [ ] **Step 4: Remove the old function body**

Delete the previous body of `RecordHomepageDeploymentFromResult` so only the wrapper and shared helper remain. Keep helper functions `ledgerString`, `firstLedgerString`, and `ledgerBuildDirFromDeployPath` unchanged.

- [ ] **Step 5: Run strict and existing deployment parsing tests**

Run:

```powershell
go test ./internal/tools/... -run "TestRecordHomepageDeploymentFromResult|TestRecordHomepageDeploymentPersists|TestRecordHomepageDeploymentKeeps" -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit strict recording API**

Run:

```powershell
git add internal/tools/homepage_ledger.go internal/tools/homepage_ledger_test.go
git commit -m "Add strict homepage deployment ledger recording"
```

---

### Task 5: Agent Dispatch Hard Failures

**Files:**
- Modify: `internal/agent/agent_dispatch_services.go`
- Modify: `internal/agent/dispatch_homepage_test.go`

- [ ] **Step 1: Write failing dispatch tests**

Add these tests after `TestDispatchHomepageInitProjectRegistersProjectDir` in `internal/agent/dispatch_homepage_test.go`:

```go
func TestDispatchHomepagePublishLocalRequiresProjectDir(t *testing.T) {
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Homepage.WorkspacePath = t.TempDir()
	cfg.Homepage.AllowLocalServer = true
	db, err := tools.InitHomepageRegistryDB(t.TempDir() + "/homepage.db")
	if err != nil {
		t.Fatalf("InitHomepageRegistryDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	output, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "homepage",
		Operation: "publish_local",
	}, &DispatchContext{Cfg: cfg, Logger: testLogger, HomepageRegistryDB: db})
	if !ok {
		t.Fatal("expected homepage operation to be handled")
	}
	if !strings.Contains(output, `"status":"error"`) || !strings.Contains(output, "project_dir is required") {
		t.Fatalf("expected missing project_dir error, got %s", output)
	}
}

func TestDispatchHomepageDeployRequiresProjectDir(t *testing.T) {
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Homepage.AllowDeploy = true
	cfg.Homepage.WorkspacePath = t.TempDir()
	db, err := tools.InitHomepageRegistryDB(t.TempDir() + "/homepage.db")
	if err != nil {
		t.Fatalf("InitHomepageRegistryDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	output, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "homepage",
		Operation: "deploy",
	}, &DispatchContext{Cfg: cfg, Logger: testLogger, HomepageRegistryDB: db})
	if !ok {
		t.Fatal("expected homepage operation to be handled")
	}
	if !strings.Contains(output, `"status":"error"`) || !strings.Contains(output, "project_dir is required") {
		t.Fatalf("expected missing project_dir error, got %s", output)
	}
}
```

- [ ] **Step 2: Run dispatch tests and confirm failure**

Run:

```powershell
go test ./internal/agent/... -run "TestDispatchHomepage(PublishLocalRequiresProjectDir|DeployRequiresProjectDir)" -count=1
```

Expected: FAIL until dispatch rejects empty `project_dir`.

- [ ] **Step 3: Add dispatch helpers**

In `internal/agent/agent_dispatch_services.go`, add these helpers near `withHomepageLedgerWarnings`:

```go
func homepageLedgerFatalResult(result string, err error) string {
	if err == nil {
		return result
	}
	message := "deployment target could not be recorded: " + err.Error()
	var parsed map[string]interface{}
	if json.Unmarshal([]byte(result), &parsed) == nil {
		parsed["status"] = "error"
		parsed["message"] = message
		out, _ := json.Marshal(parsed)
		return string(out)
	}
	return fmt.Sprintf(`{"status":"error","message":"%s"}`, strings.ReplaceAll(message, `"`, `'`))
}

func homepageProjectDirRequired(operation string) string {
	return fmt.Sprintf(`Tool Output: {"status":"error","message":"project_dir is required for homepage %s. Pass the workspace-relative project directory, for example project_dir=\"my-site\"."}`, operation)
}
```

- [ ] **Step 4: Require `project_dir` for deploy/publish operations**

In the homepage dispatch switch, add this guard before calling each operation for `deploy`, `publish_local`, `deploy_netlify`, and `deploy_vercel`:

```go
				if strings.TrimSpace(req.ProjectDir) == "" {
					return homepageProjectDirRequired(req.Operation)
				}
```

For `webserver_start`, keep existing restore/autodetect behavior, but record the returned target when the result contains `project_dir`. Do not add the required guard to `webserver_start`.

- [ ] **Step 5: Use strict deployment recording**

Replace each successful deploy recording block like:

```go
warnings := tools.RecordHomepageDeploymentFromResult(homepageCfg, homepageRegistryDB, req.ProjectDir, "local", req.BuildDir, result, logger)
result = withHomepageLedgerWarnings(result, warnings)
```

with:

```go
warnings, fatal := tools.RecordHomepageDeploymentFromResultStrict(homepageCfg, homepageRegistryDB, req.ProjectDir, "local", req.BuildDir, result, logger)
result = withHomepageLedgerWarnings(result, warnings)
if fatal != nil {
	result = homepageLedgerFatalResult(result, fatal)
}
```

Apply the same pattern for providers `sftp`, `netlify`, and `vercel`.

For unverified Netlify, keep the existing visible warning path:

```go
warnings := []string{"Netlify deploy result was not verified; deployment ledger was not updated"}
```

Do not call strict recording for unverified Netlify results.

- [ ] **Step 6: Record Caddy/local `webserver_start` target when identifiable**

In the `webserver_start` case, replace the direct return with:

```go
				result := tools.HomepageWebServerStart(homepageCfg, req.ProjectDir, req.BuildDir, logger)
				if homepageResultSuccess(result) && homepageRegistryDB != nil {
					projectDir := homepageProjectDirFromResult(result, req.ProjectDir)
					warnings, fatal := tools.RecordHomepageDeploymentFromResultStrict(homepageCfg, homepageRegistryDB, projectDir, "caddy", req.BuildDir, result, logger)
					result = withHomepageLedgerWarnings(result, warnings)
					if fatal != nil {
						result = homepageLedgerFatalResult(result, fatal)
					}
				}
				return "Tool Output: " + result
```

- [ ] **Step 7: Run dispatch tests and confirm pass**

Run:

```powershell
go test ./internal/agent/... -run "TestDispatchHomepage" -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit dispatch hard failures**

Run:

```powershell
git add internal/agent/agent_dispatch_services.go internal/agent/dispatch_homepage_test.go
git commit -m "Fail homepage deploys when ledger targets are missing"
```

---

### Task 6: Agent Schema and Tool Manuals

**Files:**
- Modify: `internal/agent/native_tools_integrations.go`
- Modify: `prompts/tools_manuals/homepage_registry.md`
- Modify: `prompts/tools_manuals/homepage.md`

- [ ] **Step 1: Update schema descriptions**

In `internal/agent/native_tools_integrations.go`, update the `homepage_registry` description from:

```go
"Track homepage/web projects, deploy history, project history, problems, metadata. Read list_history before changes; add_history after.",
```

to:

```go
"Track homepage/web projects, deploy history, project history, problems, metadata. register requires project_dir. Read list_history before changes; add_history after.",
```

Update the `project_dir` property description from:

```go
"Project directory within workspace"
```

to:

```go
"Required workspace-relative project directory for register and all project mutations"
```

Update the `homepage_deploy` `project_dir` property description from:

```go
"Project subdirectory."
```

to:

```go
"Required workspace-relative project subdirectory for deploy, publish_local, deploy_netlify, and deploy_vercel."
```

- [ ] **Step 2: Update homepage registry manual**

In `prompts/tools_manuals/homepage_registry.md`, add this paragraph after the prerequisites:

```markdown
## Identity Contract

`project_dir` is required when registering a project. It must be relative to the homepage workspace, for example `portfolio` or `sites/portfolio`, never `/workspace/portfolio`. AuraGo rejects new entries without `project_dir` so later cleanup and deployment tasks can always map a registry row back to the correct local folder.
```

In the register section, add:

```markdown
Required fields for `register`: `name`, `project_dir`.
```

- [ ] **Step 3: Update homepage manual**

In `prompts/tools_manuals/homepage.md`, add this bullet near the existing `project_dir` rules:

```markdown
- Mutating and deployment operations must carry the workspace-relative `project_dir`. Do not deploy, publish, build, or clean up a homepage project from a guessed folder name.
```

In the `publish_local` and deployment sections, ensure examples include `"project_dir": "my-site"` and no text describes `project_dir` as optional for deployment.

- [ ] **Step 4: Run docs/schema grep checks**

Run:

```powershell
rg -n "project_dir.*no|optional.*project_dir|Project subdirectory\\." prompts/tools_manuals/homepage.md prompts/tools_manuals/homepage_registry.md internal/agent/native_tools_integrations.go
```

Expected: no remaining wording that says deployment/mutation `project_dir` is optional.

- [ ] **Step 5: Commit docs and schema guidance**

Run:

```powershell
git add internal/agent/native_tools_integrations.go prompts/tools_manuals/homepage_registry.md prompts/tools_manuals/homepage.md
git commit -m "Document required homepage project identity"
```

---

### Task 7: Final Verification and DOX Closeout

**Files:**
- Review: `AGENTS.md`
- Review changed paths against Root DOX contract.

- [ ] **Step 1: Run focused test suites**

Run:

```powershell
go test ./internal/tools/... ./internal/agent/... -count=1
```

Expected: PASS.

- [ ] **Step 2: Run full Go test suite if focused tests pass**

Run:

```powershell
go test ./... -count=1
```

Expected: PASS, or report unrelated existing failures with package and error summary.

- [ ] **Step 3: Run GitNexus change detection before final commit**

Run:

```powershell
npx gitnexus detect-changes --scope all --repo 'C:\Users\Andi\Documents\repo\AuraGo'
```

Expected: output lists changed symbols in homepage registry, ledger, dispatch, manuals, and risk. If risk is HIGH or CRITICAL, stop and report the blast radius.

- [ ] **Step 4: Review staged changes for secrets**

Run:

```powershell
git diff --cached
rg -n "AURAGO_MASTER_KEY|sk-or-|password|secret" internal prompts docs --glob "!docs/superpowers/plans/2026-06-30-homepage-registry-hard-identity.md"
```

Expected: no credentials introduced. Existing documentation mentions of the words `password` or `secret` can be ignored only after reading the exact matching lines.

- [ ] **Step 5: Commit final integration if previous tasks were not committed separately**

Run:

```powershell
git status --short
git add internal/tools/homepage_validation.go internal/tools/homepage_registry.go internal/tools/homepage_ledger.go internal/agent/agent_dispatch_services.go internal/agent/native_tools_integrations.go internal/tools/homepage_test.go internal/tools/homepage_registry_test.go internal/tools/homepage_ledger_test.go internal/agent/dispatch_homepage_test.go prompts/tools_manuals/homepage_registry.md prompts/tools_manuals/homepage.md
git commit -m "Enforce hard homepage registry identity"
```

Expected: commit succeeds. If each task was committed separately, this step should find no source changes to commit.

## Spec Coverage Review

- New registry rows require `project_dir`: Tasks 1 and 2.
- Ambiguous root identity is rejected for agent-created projects: Tasks 1 and 3.
- Deploy target row is mandatory after successful deploy/publish: Tasks 3, 4, and 5.
- Local Caddy/published site becomes machine-readable through provider `caddy` or `local`: Tasks 4 and 5.
- Fatal deploy ledger failures surface to the agent as JSON errors: Task 5.
- Existing incomplete entries remain untouched: no task performs migration or cleanup.
- Agent manuals describe the new hard contract: Task 6.
- Verification and DOX closeout: Task 7.
