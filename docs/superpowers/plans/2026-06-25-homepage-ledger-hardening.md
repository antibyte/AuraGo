# Homepage Ledger Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the managed website ledger defects so AuraGo correctly detects undeployed local revisions, links deployments to the right target, records Netlify fallback deployments against the real project, and exposes deploy targets/remote observations to the API and agent context.

**Architecture:** Keep `homepage_registry.db` as the single source of truth and make the ledger read/write model internally consistent. Deployment recording will normalize project/build inputs before writing, reconciliation will compare current revision to latest successful deployment, and read models will include compact deploy target and remote observation data without breaking existing JSON fields.

**Tech Stack:** Go standard library, existing SQLite/modernc registry, existing Homepage ledger/server/agent packages, existing GitNexus/codebase-memory checks, `rtk go test`.

---

## Files

- Modify: `internal/tools/homepage_ledger.go`
  - Add read-model structs for deploy targets and remote observations.
  - Normalize deployment project/build values.
  - Fix target ID lookup after deploy target upsert.
  - Make reconciliation compare current revision with latest successful deployment revision.
  - Populate managed site details with deploy targets, recent deployments, and observations.
- Modify: `internal/tools/homepage_ledger_test.go`
  - Add regression coverage for target ID linkage, undeployed revisions, Netlify fallback normalization, target/observation read models.
- Modify: `internal/agent/tooling_policy.go`
  - Add compact target/observation data to managed-sites prompt context.
- Modify: `internal/agent/tooling_policy_test.go`
  - Assert prompt context includes provider targets and drift info.
- Modify: `internal/server/homepage_handlers_test.go`
  - Assert `/api/homepage/sites/{id}` includes deploy targets and remote observations while preserving existing fields.
- Review only: `AGENTS.md`
  - DOX pass after implementation. Update only if the stable Homepage Ledger contract changes beyond the existing section.

---

### Task 1: Pre-Edit Impact And Baseline

**Files:**
- Read: `internal/tools/homepage_ledger.go`
- Read: `internal/agent/tooling_policy.go`
- Read: `internal/server/homepage_handlers.go`
- Read: `internal/tools/homepage_ledger_test.go`

- [ ] **Step 1: Confirm worktree scope**

Run:

```powershell
rtk git status --short --branch
```

Expected: note any unrelated dirty files, especially the existing `ui/js/desktop/apps/sheets*.js` changes. Do not modify or stage unrelated files.

- [ ] **Step 2: Run GitNexus impact for touched symbols**

Run GitNexus impact before editing these symbols:

```text
impact({target:"RecordHomepageDeployment", direction:"upstream"})
impact({target:"RecordHomepageDeploymentFromResult", direction:"upstream"})
impact({target:"ReconcileHomepageProject", direction:"upstream"})
impact({target:"ListHomepageManagedSites", direction:"upstream"})
impact({target:"GetHomepageManagedSite", direction:"upstream"})
impact({target:"buildManagedSitesPromptContext", direction:"upstream"})
```

Expected: record direct callers and risk level in the implementation notes. If any result is HIGH or CRITICAL, pause and warn the user before code edits.

- [ ] **Step 3: Run current targeted tests**

Run:

```powershell
rtk go test ./internal/tools ./internal/agent ./internal/server
```

Expected: PASS before changes, or document unrelated failures before continuing.

---

### Task 2: Fix Deployment Target Linkage

**Files:**
- Modify: `internal/tools/homepage_ledger.go`
- Test: `internal/tools/homepage_ledger_test.go`

- [ ] **Step 1: Add failing test for repeated provider target upserts**

Add this test to `internal/tools/homepage_ledger_test.go` near the existing deployment tests:

```go
func TestRecordHomepageDeploymentKeepsDeploymentTargetLinkAfterUpsert(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	workspace := t.TempDir()
	cfg := HomepageConfig{WorkspacePath: workspace}
	projectA := "site-a"
	projectB := "site-b"
	for _, projectDir := range []string{projectA, projectB} {
		buildPath := filepath.Join(workspace, projectDir)
		if err := os.MkdirAll(buildPath, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", projectDir, err)
		}
		if err := os.WriteFile(filepath.Join(buildPath, "index.html"), []byte(projectDir), 0644); err != nil {
			t.Fatalf("write %s: %v", projectDir, err)
		}
	}
	projA, err := EnsureHomepageProjectForDir(db, cfg, projectA, "site-a", "html")
	if err != nil {
		t.Fatalf("ensure site-a: %v", err)
	}
	projB, err := EnsureHomepageProjectForDir(db, cfg, projectB, "site-b", "html")
	if err != nil {
		t.Fatalf("ensure site-b: %v", err)
	}
	if err := RecordHomepageDeployment(db, HomepageDeploymentRecord{
		ProjectID:        projA.ID,
		Provider:         "netlify",
		ProviderTargetID: "site-a-target",
		ProviderDeployID: "deploy-a-1",
		URL:              "https://a-one.example",
		Status:           "ok",
	}); err != nil {
		t.Fatalf("record site-a first deployment: %v", err)
	}
	if err := RecordHomepageDeployment(db, HomepageDeploymentRecord{
		ProjectID:        projB.ID,
		Provider:         "netlify",
		ProviderTargetID: "site-b-target",
		ProviderDeployID: "deploy-b-1",
		URL:              "https://b.example",
		Status:           "ok",
	}); err != nil {
		t.Fatalf("record site-b deployment: %v", err)
	}
	if err := RecordHomepageDeployment(db, HomepageDeploymentRecord{
		ProjectID:        projA.ID,
		Provider:         "netlify",
		ProviderTargetID: "site-a-target",
		ProviderDeployID: "deploy-a-2",
		URL:              "https://a-two.example",
		Status:           "ok",
	}); err != nil {
		t.Fatalf("record site-a second deployment: %v", err)
	}

	var linkedProjectID int64
	err = db.QueryRow(`SELECT t.project_id
		FROM homepage_deployments d
		JOIN homepage_deploy_targets t ON t.id = d.target_id
		WHERE d.provider_deploy_id = 'deploy-a-2'`).Scan(&linkedProjectID)
	if err != nil {
		t.Fatalf("read linked target project: %v", err)
	}
	if linkedProjectID != projA.ID {
		t.Fatalf("deploy-a-2 linked to project %d, want %d", linkedProjectID, projA.ID)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```powershell
rtk go test ./internal/tools -run TestRecordHomepageDeploymentKeepsDeploymentTargetLinkAfterUpsert
```

Expected before fix: FAIL if SQLite returns a stale `LastInsertId()` for the upsert update path, or PASS on a driver version that masks the bug. Continue with the implementation either way because the code path is still unsafe.

- [ ] **Step 3: Replace `LastInsertId()` target lookup**

In `RecordHomepageDeployment`, replace the `LastInsertId()` branch with an explicit lookup after the upsert:

```go
	var targetID int64
	if err := db.QueryRow(
		"SELECT id FROM homepage_deploy_targets WHERE project_id = ? AND provider = ?",
		rec.ProjectID,
		rec.Provider,
	).Scan(&targetID); err != nil {
		return fmt.Errorf("failed to read deploy target id: %w", err)
	}
```

Keep the existing `targetIDValue` null handling after this block.

- [ ] **Step 4: Run target linkage tests**

Run:

```powershell
rtk go test ./internal/tools -run "RecordHomepageDeployment"
```

Expected: PASS.

---

### Task 3: Mark New Local Revisions As Not Deployed

**Files:**
- Modify: `internal/tools/homepage_ledger.go`
- Test: `internal/tools/homepage_ledger_test.go`

- [ ] **Step 1: Add failing test for revision newer than deployment**

Add this test to `internal/tools/homepage_ledger_test.go`:

```go
func TestReconcileHomepageProjectDetectsUndeployedLatestRevision(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	workspace := t.TempDir()
	projectDir := "site-a"
	projectPath := filepath.Join(workspace, projectDir)
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	cfg := HomepageConfig{WorkspacePath: workspace}
	if err := os.WriteFile(filepath.Join(projectPath, "index.html"), []byte("one"), 0644); err != nil {
		t.Fatalf("write initial file: %v", err)
	}
	proj, err := EnsureHomepageProjectForDir(db, cfg, projectDir, "site-a", "html")
	if err != nil {
		t.Fatalf("ensure project: %v", err)
	}
	initial := SaveHomepageRevisionAndState(cfg, db, projectDir, "initial", "test", "test", nil, slog.Default())
	if len(initial.Warnings) > 0 || initial.RevisionID == 0 {
		t.Fatalf("initial revision = %#v", initial)
	}
	if err := RecordHomepageDeployment(db, HomepageDeploymentRecord{
		ProjectID:  proj.ID,
		RevisionID: initial.RevisionID,
		Provider:   "local",
		URL:        "",
		BuildDir:   ".",
		Status:     "ok",
	}); err != nil {
		t.Fatalf("record initial deploy: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectPath, "index.html"), []byte("two"), 0644); err != nil {
		t.Fatalf("write changed file: %v", err)
	}
	changed := SaveHomepageRevisionAndState(cfg, db, projectDir, "changed", "test", "test", nil, slog.Default())
	if len(changed.Warnings) > 0 || changed.RevisionID == 0 {
		t.Fatalf("changed revision = %#v", changed)
	}

	state, err := ReconcileHomepageProject(cfg, db, projectDir, slog.Default())
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if state.DriftStatus != "not_deployed" {
		t.Fatalf("DriftStatus = %q, want not_deployed; message=%q", state.DriftStatus, state.DriftMessage)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```powershell
rtk go test ./internal/tools -run TestReconcileHomepageProjectDetectsUndeployedLatestRevision
```

Expected before fix: FAIL with `DriftStatus = "clean"` or another non-`not_deployed` state.

- [ ] **Step 3: Add latest deployed revision helper**

Add a small unexported helper to `internal/tools/homepage_ledger.go` near `ListHomepageManagedDeployments`:

```go
func latestSuccessfulHomepageDeploymentRevision(db *sql.DB, projectID int64) int64 {
	var revision sql.NullInt64
	_ = db.QueryRow(`SELECT revision_id
		FROM homepage_deployments
		WHERE project_id = ? AND status NOT IN ('error', 'failed') AND revision_id IS NOT NULL
		ORDER BY created_at DESC, id DESC LIMIT 1`, projectID).Scan(&revision)
	if revision.Valid {
		return revision.Int64
	}
	return 0
}
```

- [ ] **Step 4: Compare current revision to latest deployment in reconciliation**

In `ReconcileHomepageProject`, after the local file-state comparison and before remote observation, insert this logic:

```go
	if drift == "clean" {
		state, stateErr := GetHomepageProjectState(db, proj.ID)
		if stateErr == nil && state.CurrentRevisionID > 0 {
			latestDeployedRevision := latestSuccessfulHomepageDeploymentRevision(db, proj.ID)
			if latestDeployedRevision == 0 {
				drift = "not_deployed"
				message = "current revision has not been deployed"
			} else if latestDeployedRevision < state.CurrentRevisionID {
				drift = "not_deployed"
				message = fmt.Sprintf("current revision %d is newer than deployed revision %d", state.CurrentRevisionID, latestDeployedRevision)
			}
		}
	}
```

Keep the existing `COUNT(*) FROM homepage_deployments` fallback, but only run it if `drift == "clean"` after this new revision check.

- [ ] **Step 5: Run reconciliation tests**

Run:

```powershell
rtk go test ./internal/tools -run "ReconcileHomepageProject"
```

Expected: PASS.

---

### Task 4: Normalize Netlify Fallback Deployment Inputs

**Files:**
- Modify: `internal/tools/homepage_ledger.go`
- Test: `internal/tools/homepage_ledger_test.go`

- [ ] **Step 1: Add failing test for Netlify fallback project/build**

Add this test to `internal/tools/homepage_ledger_test.go`:

```go
func TestRecordHomepageDeploymentFromResultUsesNetlifyFallbackProjectAndBuildDir(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	workspace := t.TempDir()
	cfg := HomepageConfig{WorkspacePath: workspace}
	originalDir := "next-site"
	fallbackDir := "next-site-static"
	buildDir := "public"
	buildPath := filepath.Join(workspace, fallbackDir, buildDir)
	if err := os.MkdirAll(buildPath, 0755); err != nil {
		t.Fatalf("mkdir fallback build: %v", err)
	}
	if err := os.WriteFile(filepath.Join(buildPath, "index.html"), []byte("<h1>fallback</h1>"), 0644); err != nil {
		t.Fatalf("write fallback build: %v", err)
	}
	if _, err := EnsureHomepageProjectForDir(db, cfg, originalDir, "next-site", "next"); err != nil {
		t.Fatalf("ensure original project: %v", err)
	}
	result := `{"status":"ok","deploy_id":"deploy-1","site_id":"site-1","verified_url":"https://fallback.example","fallback_project_dir":"next-site-static","fallback_build_dir":"public"}`

	warnings := RecordHomepageDeploymentFromResult(cfg, db, originalDir, "netlify", "", result, slog.Default())
	if len(warnings) > 0 {
		t.Fatalf("warnings: %v", warnings)
	}
	var projectDir, build string
	err := db.QueryRow(`SELECT p.project_dir, d.build_dir
		FROM homepage_deployments d
		JOIN homepage_projects p ON p.id = d.project_id
		WHERE d.provider_deploy_id = 'deploy-1'`).Scan(&projectDir, &build)
	if err != nil {
		t.Fatalf("read deployment: %v", err)
	}
	if projectDir != fallbackDir || build != buildDir {
		t.Fatalf("projectDir=%q buildDir=%q, want %q/%q", projectDir, build, fallbackDir, buildDir)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```powershell
rtk go test ./internal/tools -run TestRecordHomepageDeploymentFromResultUsesNetlifyFallbackProjectAndBuildDir
```

Expected before fix: FAIL because the deployment is recorded against `next-site` or build dir `.`.

- [ ] **Step 3: Normalize project/build before ensuring project**

At the start of `RecordHomepageDeploymentFromResult`, after JSON parse and successful status check, add:

```go
	if fallbackProjectDir := ledgerString(parsed, "fallback_project_dir"); fallbackProjectDir != "" {
		projectDir = fallbackProjectDir
	}
	if fallbackBuildDir := ledgerString(parsed, "fallback_build_dir"); fallbackBuildDir != "" {
		buildDir = fallbackBuildDir
	}
	if parsedProjectDir := ledgerString(parsed, "project_dir"); parsedProjectDir != "" && provider != "netlify" {
		projectDir = parsedProjectDir
	}
```

Move the existing `EnsureHomepageProjectForDir` call to happen after this normalization. Keep `project_dir` handling conservative for Netlify because fallback data is more specific than the original request.

- [ ] **Step 4: Run deployment parsing tests**

Run:

```powershell
rtk go test ./internal/tools -run "RecordHomepageDeploymentFromResult|RecordHomepageDeployment"
```

Expected: PASS.

---

### Task 5: Expose Targets And Remote Observations In Read Models

**Files:**
- Modify: `internal/tools/homepage_ledger.go`
- Test: `internal/tools/homepage_ledger_test.go`
- Test: `internal/server/homepage_handlers_test.go`

- [ ] **Step 1: Add read-model structs**

Add these types near `HomepageManagedDeployment`:

```go
type HomepageManagedDeployTarget struct {
	ID               int64  `json:"id"`
	Provider         string `json:"provider"`
	ProviderTargetID string `json:"provider_target_id,omitempty"`
	URL              string `json:"url,omitempty"`
	RemotePath       string `json:"remote_path,omitempty"`
	LastSeenAt       string `json:"last_seen_at,omitempty"`
	UpdatedAt        string `json:"updated_at,omitempty"`
}

type HomepageManagedRemoteObservation struct {
	ID               int64  `json:"id"`
	TargetID         int64  `json:"target_id,omitempty"`
	ObservedAt       string `json:"observed_at"`
	Provider         string `json:"provider"`
	URL              string `json:"url,omitempty"`
	ProviderDeployID string `json:"provider_deploy_id,omitempty"`
	ContentHash      string `json:"content_hash,omitempty"`
	Status           string `json:"status"`
}
```

Extend `HomepageManagedSite`:

```go
	DeployTargets      []HomepageManagedDeployTarget      `json:"deploy_targets,omitempty"`
	RemoteObservations []HomepageManagedRemoteObservation `json:"remote_observations,omitempty"`
```

- [ ] **Step 2: Add list helpers**

Add these helpers after `ListHomepageManagedDeployments`:

```go
func ListHomepageManagedDeployTargets(db *sql.DB, projectID int64) ([]HomepageManagedDeployTarget, error) {
	rows, err := db.Query(`SELECT id, provider, provider_target_id, url, remote_path,
			COALESCE(last_seen_at, ''), COALESCE(updated_at, '')
		FROM homepage_deploy_targets
		WHERE project_id = ?
		ORDER BY updated_at DESC, id DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var targets []HomepageManagedDeployTarget
	for rows.Next() {
		var target HomepageManagedDeployTarget
		if err := rows.Scan(&target.ID, &target.Provider, &target.ProviderTargetID, &target.URL, &target.RemotePath, &target.LastSeenAt, &target.UpdatedAt); err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, rows.Err()
}

func ListHomepageManagedRemoteObservations(db *sql.DB, projectID int64, limit int) ([]HomepageManagedRemoteObservation, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := db.Query(`SELECT id, COALESCE(target_id, 0), observed_at, provider, url,
			provider_deploy_id, content_hash, status
		FROM homepage_remote_observations
		WHERE project_id = ?
		ORDER BY observed_at DESC, id DESC
		LIMIT ?`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var observations []HomepageManagedRemoteObservation
	for rows.Next() {
		var obs HomepageManagedRemoteObservation
		if err := rows.Scan(&obs.ID, &obs.TargetID, &obs.ObservedAt, &obs.Provider, &obs.URL, &obs.ProviderDeployID, &obs.ContentHash, &obs.Status); err != nil {
			return nil, err
		}
		observations = append(observations, obs)
	}
	return observations, rows.Err()
}
```

- [ ] **Step 3: Populate details in `GetHomepageManagedSite`**

After deployments are loaded, load targets and observations:

```go
			targets, err := ListHomepageManagedDeployTargets(db, id)
			if err != nil {
				return HomepageManagedSite{}, err
			}
			observations, err := ListHomepageManagedRemoteObservations(db, id, 10)
			if err != nil {
				return HomepageManagedSite{}, err
			}
			site.Deployments = deployments
			site.DeployTargets = targets
			site.RemoteObservations = observations
			return site, nil
```

- [ ] **Step 4: Add tools read-model test**

Add a test that records a deployment, runs `ReconcileHomepageProject` twice against an `httptest.Server`, and then asserts:

```go
site, err := GetHomepageManagedSite(db, proj.ID)
if err != nil {
	t.Fatalf("managed site: %v", err)
}
if len(site.DeployTargets) != 1 {
	t.Fatalf("DeployTargets len = %d, want 1", len(site.DeployTargets))
}
if len(site.RemoteObservations) == 0 {
	t.Fatal("RemoteObservations should be populated")
}
```

- [ ] **Step 5: Add server API response test**

In `internal/server/homepage_handlers_test.go`, extend the existing homepage sites test or add a new one that calls `GET /api/homepage/sites/{id}` and asserts the JSON contains:

```go
if !strings.Contains(body, `"deploy_targets"`) {
	t.Fatalf("response missing deploy_targets: %s", body)
}
if !strings.Contains(body, `"remote_observations"`) {
	t.Fatalf("response missing remote_observations: %s", body)
}
```

- [ ] **Step 6: Run tools/server tests**

Run:

```powershell
rtk go test ./internal/tools -run "HomepageManaged|RemoteObservations|ReconcileHomepageProject"
rtk go test ./internal/server -run "Homepage"
```

Expected: PASS.

---

### Task 6: Add Targets To Agent Managed-Sites Context

**Files:**
- Modify: `internal/agent/tooling_policy.go`
- Test: `internal/agent/tooling_policy_test.go`

- [ ] **Step 1: Add prompt context test**

Extend `TestBuildPromptContextFlagsInjectsManagedSitesContext` or add a new test that creates a project, records a deployment target, builds prompt flags, and asserts:

```go
if !strings.Contains(flags.ReuseContext, "netlify=") {
	t.Fatalf("ReuseContext missing deploy target provider: %q", flags.ReuseContext)
}
if !strings.Contains(flags.ReuseContext, "remote=") {
	t.Fatalf("ReuseContext missing remote observation summary: %q", flags.ReuseContext)
}
```

- [ ] **Step 2: Load detailed site data in prompt builder**

In `buildManagedSitesPromptContext`, replace the per-site formatting block with detailed loading:

```go
		detailed, detailErr := tools.GetHomepageManagedSite(runCfg.HomepageRegistryDB, site.ID)
		if detailErr == nil {
			site = detailed
		}
```

Then append compact target and observation summaries:

```go
		if len(site.DeployTargets) > 0 {
			var targets []string
			for _, target := range site.DeployTargets {
				value := target.URL
				if value == "" {
					value = target.RemotePath
				}
				if value == "" {
					value = target.ProviderTargetID
				}
				if value != "" {
					targets = append(targets, fmt.Sprintf("%s=%s", target.Provider, value))
				} else {
					targets = append(targets, target.Provider)
				}
			}
			line += ", targets=" + strings.Join(targets, "|")
		}
		if len(site.RemoteObservations) > 0 {
			obs := site.RemoteObservations[0]
			line += fmt.Sprintf(", remote=%s/%s", obs.Provider, obs.Status)
			if obs.ObservedAt != "" {
				line += fmt.Sprintf("@%s", obs.ObservedAt)
			}
		}
```

- [ ] **Step 3: Run agent prompt tests**

Run:

```powershell
rtk go test ./internal/agent -run "Homepage|NativeTools|PromptContext"
```

Expected: PASS.

---

### Task 7: Full Verification, DOX Pass, And Commit

**Files:**
- Review: `AGENTS.md`
- Review: all modified files

- [ ] **Step 1: Run targeted package tests**

Run:

```powershell
rtk go test ./internal/tools ./internal/agent ./internal/server
```

Expected: PASS.

- [ ] **Step 2: Run formatting**

Run:

```powershell
rtk gofmt -w internal/tools/homepage_ledger.go internal/tools/homepage_ledger_test.go internal/agent/tooling_policy.go internal/agent/tooling_policy_test.go internal/server/homepage_handlers_test.go
```

Expected: no output.

- [ ] **Step 3: DOX pass**

Re-check changed paths against `AGENTS.md`.

Expected: root `AGENTS.md` already documents Homepage Managed Website Ledger semantics. Update it only if the implementation changes the stable contract, for example if the public API contract gains newly guaranteed `deploy_targets` and `remote_observations` fields that should be durable documentation.

- [ ] **Step 4: Run GitNexus change detection**

Run GitNexus:

```text
detect_changes({scope:"compare", base_ref:"main"})
```

Expected: changes limited to Homepage ledger, agent managed-sites context, and tests. Investigate any unrelated execution-flow changes before commit.

- [ ] **Step 5: Review diff and secrets**

Run:

```powershell
rtk git diff -- internal/tools/homepage_ledger.go internal/tools/homepage_ledger_test.go internal/agent/tooling_policy.go internal/agent/tooling_policy_test.go internal/server/homepage_handlers_test.go AGENTS.md
rtk git status --short
```

Expected: no secrets, no unrelated `ui/js/desktop/apps/sheets*.js` files staged.

- [ ] **Step 6: Stage only intended files**

Run:

```powershell
rtk git add internal/tools/homepage_ledger.go internal/tools/homepage_ledger_test.go internal/agent/tooling_policy.go internal/agent/tooling_policy_test.go internal/server/homepage_handlers_test.go
```

If `AGENTS.md` was changed during the DOX pass, also run:

```powershell
rtk git add AGENTS.md
```

- [ ] **Step 7: Commit**

Run:

```powershell
rtk git commit -m "fix: harden managed website ledger consistency"
```

Expected: commit succeeds and excludes unrelated dirty files.

---

## Self-Review

- Spec coverage: fixes all four review findings: undeployed revision drift, deployment target foreign key linkage, Netlify fallback normalization, and missing target/observation visibility in API/agent context.
- Placeholder scan: no task contains a deferred implementation placeholder; each code-changing task includes concrete snippets and exact test commands.
- Type consistency: new exported types use `HomepageManaged*` naming in `internal/tools`, and field names match the planned JSON additions used by server/agent tests.
